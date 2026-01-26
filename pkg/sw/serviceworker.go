package sw

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/serviceworker"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

type swProbe struct {
	url     string
	cidv0   cid.Cid
	cidv1   cid.Cid
	cdpHost string
	cdpPort int

	listenMu sync.Mutex

	// Helia's configuration extracted from the query parameter during redirects.
	// as the redirect logic changes this may not be parsed correctly in the
	// future, so this field is set on a best effort.
	heliaConfig *HeliaConfig

	// Sets of trustless gateway and delegated router hosts. These sets
	// are prepopulated with "trustless-gateway.link" and "delegated-ipfs.dev"
	// but then also enriched with any entry that's parsed from the Helia config.
	trustlessGateways map[string]struct{}
	delegatedRouters  map[string]struct{}

	// Maps of trustless gateway and delegated router responses (these are
	// tracked in the context of the service worker target).
	trustlessGatewayRequests map[network.RequestID]*network.EventResponseReceived
	delegatedRouterRequests  map[network.RequestID]*NetworkEventResponse

	// The response bodies are requested asynchronously. This waitgroup
	// completes once the go routines have finished.
	responseReqWg sync.WaitGroup

	// Page requests for "documents" (these are tracked in the context of the
	// page and capture the redirect flow). That's also why we need to fields;
	// the first documentRequestIDs captures the order and the documentRequests
	// field the payload
	documentRequestIDs []network.RequestID
	documentRequests   map[network.RequestID]*swRequestTrace

	navigationEvents []*page.EventFrameNavigated
	lifecycleEvents  map[cdp.LoaderID][]*page.EventLifecycleEvent

	idleWaitLogOnce sync.Once
}
type NetworkEventResponse struct {
	*network.EventResponseReceived
	RequestFinished bool
	Canceled        bool
	Body            []byte
}

func NewSwProbe(c cid.Cid, url string, cdpHost string, cdpPort int) *swProbe {
	return &swProbe{
		cidv0:   cid.NewCidV0(c.Hash()),
		cidv1:   cid.NewCidV1(c.Type(), c.Hash()),
		url:     url,
		cdpHost: cdpHost,
		cdpPort: cdpPort,

		trustlessGateways: map[string]struct{}{"trustless-gateway.link": {}},
		delegatedRouters:  map[string]struct{}{"delegated-ipfs.dev": {}},

		delegatedRouterRequests:  map[network.RequestID]*NetworkEventResponse{},
		trustlessGatewayRequests: map[network.RequestID]*network.EventResponseReceived{},

		documentRequestIDs: make([]network.RequestID, 0),
		documentRequests:   make(map[network.RequestID]*swRequestTrace),
		navigationEvents:   make([]*page.EventFrameNavigated, 0),
		lifecycleEvents:    map[cdp.LoaderID][]*page.EventLifecycleEvent{},
	}
}

type swProbeResult struct {
	// Core timing metrics
	TotalTTFB           time.Duration // Time to first byte including all redirects
	FinalTTFB           time.Duration // Time to first byte on final request
	TimeToFinalRedirect time.Duration // From first request to final request start

	// Service worker metadata
	ServiceWorkerVersion string // From "server" header

	// Final request details
	FinalStatusCode int
	ContentType     string
	ContentLength   int64

	// IPFS-specific headers (from final response)
	IPFSPath  string // x-ipfs-path
	IPFSRoots string // x-ipfs-roots

	// Whether any delegated router returned any providers
	FoundProviders    int
	ServedFromGateway bool

	// Server timing data (from final response)
	ServerTimings        map[string]ServerTiming
	DelegatedRouterTTFB  time.Duration
	TrustlessGatewayTTFB time.Duration

	DelegatedRouterStatus  int
	TrustlessGatewayStatus int
}

func (r *swProbe) isProbeDone() bool {
	// first check if we have seen the final request
	// for the content from the service worker gateway
	var finalRequest *swRequestTrace
	for _, req := range r.documentRequests {
		if req.isFinalRequest() {
			finalRequest = req
			break
		}
	}

	// if we haven't seen the final request yet, we're not done
	if finalRequest == nil {
		return false
	}

	finalResp := finalRequest.lastResponse()
	// if the final request has no response yet, we're not done
	if finalResp == nil {
		return false
	}

	for _, e := range r.delegatedRouterRequests {
		if e.Body == nil {
			return false
		}
	}

	// if the final response is an attachment or inline, we're done
	if cd, ok := finalResp.Headers["content-disposition"].(string); ok {
		if strings.Contains(strings.ToLower(cd), "attachment") {
			return true
		} else if strings.Contains(strings.ToLower(cd), "inline") {
			return true
		}
	}

	// Then wait until the network is idle in the final navigation
	lifecycleEvents, found := r.lifecycleEvents[finalRequest.loaderID]
	if !found {
		return false
	}

	r.idleWaitLogOnce.Do(func() {
		slog.Info("Waiting for network idle in final navigation")
	})

	for _, e := range lifecycleEvents {
		if e.Name == "networkIdle" {
			return true
		}
	}
	return false
}

// swRequestTrace represents a trace of network requests and associated
//
//	responses, including all redirects.
type swRequestTrace struct {
	currentURL string              // currentURL represents the most recent URL in the network request trace.
	responses  []*network.Response // responses contains all network responses in the trace, including redirects.
	loaderID   cdp.LoaderID        // loaderID represents the unique LoaderID associated with the request trace.
}

// lastResponse returns the last response in the trace or nil if no responses exist.
func (r *swRequestTrace) lastResponse() *network.Response {
	respCount := len(r.responses)
	if respCount == 0 {
		return nil
	}

	return r.responses[respCount-1]
}

// isFinalRequest determines whether the last response meets specific conditions.
// It checks if the response is served by a service worker, is not a redirect,
// and includes required headers "x-ipfs-path" and "x-ipfs-roots".
// Returns true if all conditions are satisfied, otherwise false.
func (r *swRequestTrace) isFinalRequest() bool {
	// get last traces response
	finalResp := r.lastResponse()
	if finalResp == nil {
		return false
	}

	// we expect the response to be from a service worker
	if !finalResp.FromServiceWorker {
		return false
	}

	//// the response must not be a redirect
	//if finalResp.Status >= 300 && finalResp.Status < 400 {
	//	return false
	//}

	// To detect the final response from the service worker, we're checking if
	// the access-control-expose-headers header is present and contains the
	// x-ipfs-path and x-ipfs-roots headers. This access-control-expose-headers
	// header is set by the service worker gateway in both cases: success
	// and failure. Previously, we were just matching on the "x-ipfs-path" and
	// "x-ipfs-roots" headers. But they are not present in case of an error.

	// in case of a 200 the headers x-ipfs-path and x-ipfs-roots are also set
	// in case of a 502 the x-ipfs-path header is also present
	// in case of a 504 only the access-control-expose-headers header is present
	xIPFSPathHeader := "x-ipfs-path"
	xIPFSRootsHeader := "x-ipfs-roots"
	accessControlExposeHeadersHeader := "access-control-expose-headers"

	accessControlExposeHeadersRaw, ok := finalResp.Headers[accessControlExposeHeadersHeader]
	if !ok {
		return false
	}

	accessControlExposeHeadersStr, ok := accessControlExposeHeadersRaw.(string)
	if !ok {
		return false
	}

	lower := strings.ToLower(accessControlExposeHeadersStr)
	containsAccessControlExposeHeadersValues := strings.Contains(lower, xIPFSPathHeader) && strings.Contains(lower, xIPFSRootsHeader)

	_, containsXIPFSPathHeader := finalResp.Headers[xIPFSPathHeader]
	_, containsXIPFSRootsHeader := finalResp.Headers[xIPFSRootsHeader]

	if containsAccessControlExposeHeadersValues || containsXIPFSPathHeader || containsXIPFSRootsHeader {
		return true
	}

	return false
}

func (p *swProbe) Run(ctx context.Context) (*swProbeResult, error) {
	browserURL := url.URL{
		Scheme: "ws",
		Host:   net.JoinHostPort(p.cdpHost, strconv.Itoa(p.cdpPort)),
	}

	slog.With("url", browserURL.String()).Debug("Connecting to browser...")
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, browserURL.String())
	defer allocCancel()

	// Create a new browser context with incognito mode.
	// chromedp.WithNewBrowserContext() forces the creation of a separate
	// ephemeral browser context (incognito) on the remote browser.
	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithNewBrowserContext())
	defer browserCancel()

	// Set up event listeners - dispatch to handler methods
	listenCtx, listenCancel := context.WithCancel(browserCtx)
	defer listenCancel()

	chromedp.ListenTarget(listenCtx, func(ev interface{}) {
		p.listenMu.Lock()
		defer p.listenMu.Unlock()

		// Dispatch to the appropriate handler
		switch e := ev.(type) {
		case *page.EventFrameNavigated:
			p.handleFrameNavigated(e)
		case *page.EventLifecycleEvent:
			p.handleLifecycleEvent(e)
		case *network.EventRequestWillBeSent:
			p.handleRequestWillBeSent(e)
		case *network.EventResponseReceived:
			p.handleResponseReceived(e)
		case *target.EventAttachedToTarget:
			go p.handleAttachedToTarget(browserCtx, e)
		}

		if p.isProbeDone() {
			slog.Debug("Probe completed")
			listenCancel()
		}
	})

	// Enable domains and start navigation
	err := chromedp.Run(browserCtx,
		network.Enable(),
		page.Enable(),
		runtime.Enable(),
		serviceworker.Enable(),
		page.SetLifecycleEventsEnabled(true),
		target.SetAutoAttach(true, false).WithFlatten(false),
		chromedp.Navigate(p.url),
	)
	if err != nil {
		return nil, fmt.Errorf("enabling domains and navigation: %w", err)
	}

	// Wait for completion or timeout
	<-listenCtx.Done()
	p.responseReqWg.Wait()

	switch ctx.Err() {
	case context.DeadlineExceeded:
		slog.Warn("Measurement timed out")
	case context.Canceled:
		slog.Warn("Measurement canceled")
	case nil:
		slog.Debug("Measurement completed successfully")
	default:
		slog.Warn("Measurement failed", "err", ctx.Err())
	}

	// Build comprehensive probe result
	return p.buildProbeResult(), ctx.Err()
}

// handleFrameNavigated processes page navigation events
func (r *swProbe) handleFrameNavigated(e *page.EventFrameNavigated) {
	slog.Debug("Navigated to "+e.Frame.URL, "loaderID", e.Frame.LoaderID)
	r.navigationEvents = append(r.navigationEvents, e)
}

// handleLifecycleEvent processes page lifecycle events
func (r *swProbe) handleLifecycleEvent(e *page.EventLifecycleEvent) {
	if e.Name == "networkIdle" {
		slog.Info("Lifecycle event", "name", e.Name)
	} else {
		slog.Debug("Lifecycle event", "name", e.Name)
	}

	events, found := r.lifecycleEvents[e.LoaderID]
	if !found {
		events = make([]*page.EventLifecycleEvent, 0)
	}

	r.lifecycleEvents[e.LoaderID] = append(events, e)
}

// handleAttachedToTarget sets up a listener for the new Service Worker target
// and tries to capture the response body for any delegated routing requests.
// In theory there could be more than one delegated router be configured. In
// practice, only one is supported but we still try to handle it here.
func (p *swProbe) handleAttachedToTarget(ctx context.Context, e *target.EventAttachedToTarget) {
	// Only care about service workers
	if e.TargetInfo.Type != "service_worker" {
		return
	}

	slog.Info("Service Worker Attached", "targetID", e.TargetInfo.TargetID)

	// This creates a context specifically addressing the Service Worker target.
	swCtx, cancel := chromedp.NewContext(ctx, chromedp.WithTargetID(e.TargetInfo.TargetID))
	defer cancel()

	// Enable the Network domain with buffering on this Service Worker
	if err := chromedp.Run(swCtx, network.Enable()); err != nil {
		slog.Error("Failed to enable network on SW", "err", err)
		return
	}

	// Attach the main event handler to this Service Worker context
	chromedp.ListenTarget(swCtx, func(ev interface{}) {
		p.listenMu.Lock()
		defer p.listenMu.Unlock()

		// Dispatch to the appropriate handler
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e.Type != network.ResourceTypeFetch {
				return
			}

			u, err := url.Parse(e.Response.URL)
			if err != nil {
				return
			}

			if _, ok := p.delegatedRouters[u.Host]; ok {
				if !strings.Contains(u.Path, "routing/v1/providers") {
					return
				}

				p.delegatedRouterRequests[e.RequestID] = &NetworkEventResponse{EventResponseReceived: e}
			} else if _, ok := p.trustlessGateways[u.Host]; ok {
				if strings.Contains(u.Path, p.cidv0.String()) || strings.Contains(u.Path, p.cidv1.String()) {
					p.trustlessGatewayRequests[e.RequestID] = e
				}
			}

		case *network.EventLoadingFailed:
			respEvent, found := p.delegatedRouterRequests[e.RequestID]
			if !found {
				return
			}

			respEvent.RequestFinished = true
			respEvent.Canceled = e.Canceled

		case *network.EventLoadingFinished:
			// Launch in goroutine as required by ListenTarget
			p.responseReqWg.Add(1)
			go func(requestID network.RequestID) {
				defer p.responseReqWg.Done()

				p.listenMu.Lock()
				// Try to get the body for tracked requests
				respEvent, found := p.delegatedRouterRequests[e.RequestID]
				if !found {
					p.listenMu.Unlock()
					return
				}

				respEvent.RequestFinished = true
				p.listenMu.Unlock()

				body, err := network.GetResponseBody(requestID).Do(cdp.WithExecutor(ctx, chromedp.FromContext(swCtx).Target))
				if err != nil {
					return
				}

				p.listenMu.Lock()
				respEvent.Body = []byte(body)
				p.listenMu.Unlock()

				slog.Info("Captured response body",
					"requestID", requestID,
					"url", respEvent.Response.URL,
					"size", len(body))
			}(e.RequestID)
		}
	})

	// Keep this goroutine alive to maintain the context/listener
	<-swCtx.Done()
}

// buildProbeResult constructs the complete probe result from collected data
// It aggregates metrics from document requests, trustless gateway fetches, and delegated router queries.
func (p *swProbe) buildProbeResult() *swProbeResult {
	p.listenMu.Lock()
	defer p.listenMu.Unlock()

	result := &swProbeResult{
		ServerTimings:        make(map[string]ServerTiming),
		DelegatedRouterTTFB:  0,
		TrustlessGatewayTTFB: 0,
	}

	providers := map[peer.ID]types.PeerRecord{}
	for _, resp := range p.delegatedRouterRequests {
		// Determine the effective status code for this response
		status := int(resp.Response.Status)
		if resp.Canceled {
			status = 499
		}

		// Apply precedence rules:
		// 1. If we already have a Success (200-299), never overwrite it.
		// 2. If the new status is Success, take it (overwrites any previous Error/Cancel).
		// 3. If we have no status yet, take the new one.
		// 4. If we have an existing Error/Cancel, only overwrite if the new one is a real Error (not 499).
		//    (This prevents a generic "Canceled" from hiding a specific HTTP error like 502).
		alreadySuccess := result.DelegatedRouterStatus >= 200 && result.DelegatedRouterStatus < 300
		newSuccess := status >= 200 && status < 300

		if !alreadySuccess {
			if newSuccess {
				result.DelegatedRouterStatus = status
			} else if result.DelegatedRouterStatus == 0 {
				result.DelegatedRouterStatus = status
			} else if status != 499 {
				result.DelegatedRouterStatus = status
			}
		}

		dec := json.NewDecoder(bytes.NewBuffer(resp.Body))
		for dec.More() {
			pr := types.PeerRecord{}
			if err := dec.Decode(&pr); err != nil {
				continue
			}

			if pr.ID == nil {
				continue
			}

			providers[*pr.ID] = pr
		}

		ttfb := time.Duration(resp.Response.Timing.ReceiveHeadersStart * float64(time.Millisecond))
		if result.DelegatedRouterTTFB == 0 || ttfb < result.DelegatedRouterTTFB {
			result.DelegatedRouterTTFB = ttfb
		}
	}

	if len(p.delegatedRouterRequests) > 0 {
		result.FoundProviders = len(providers)
	} else {
		result.FoundProviders = -1
	}

	for _, resp := range p.trustlessGatewayRequests {
		// if result.TrustlessGatewayStatus is 0 or outside the HTTP success range, set it to the current response status
		// basically give precedence to the first successful response
		if result.TrustlessGatewayStatus == 0 || result.TrustlessGatewayStatus < 200 || result.TrustlessGatewayStatus >= 300 {
			result.TrustlessGatewayStatus = int(resp.Response.Status)
		}

		if resp.Response.Status < 200 || resp.Response.Status >= 300 {
			continue
		}

		// If there are multiple successful retrievals, track the fastest one
		result.ServedFromGateway = true
		ttfb := time.Duration(resp.Response.Timing.ReceiveHeadersStart * float64(time.Millisecond))
		if result.TrustlessGatewayTTFB == 0 || ttfb < result.TrustlessGatewayTTFB {
			result.TrustlessGatewayTTFB = ttfb
		}
	}

	// Find first and final requests
	var firstReq *swRequestTrace
	var finalReq *swRequestTrace

	for _, reqID := range p.documentRequestIDs {
		req := p.documentRequests[reqID]
		if firstReq == nil {
			firstReq = req
		}

		if req.isFinalRequest() {
			finalReq = req
			break
		}
	}

	if firstReq == nil {
		slog.Warn("No requests found in probe")
		return result
	}

	if finalReq == nil {
		slog.Warn("No final request found in probe")
		return result
	}

	if len(firstReq.responses) == 0 {
		slog.Warn("First request has no responses")
		return result
	}

	firstResp := firstReq.responses[0]
	finalResp := finalReq.lastResponse()

	if firstResp == nil {
		slog.Warn("First request has no response")
		return result
	}

	if finalResp == nil {
		slog.Warn("Final request has no response")
		return result
	}

	// Extract metadata from final response
	result.FinalStatusCode = int(finalResp.Status)

	// Extract IPFS headers
	if ipfsPath, ok := finalResp.Headers["x-ipfs-path"].(string); ok {
		result.IPFSPath = ipfsPath
	}

	if ipfsRoots, ok := finalResp.Headers["x-ipfs-roots"].(string); ok {
		result.IPFSRoots = ipfsRoots
	}

	// Extract service worker version from server header
	if server, ok := finalResp.Headers["server"].(string); ok {
		result.ServiceWorkerVersion = server
	}

	// Extract content metadata
	if contentType, ok := finalResp.Headers["content-type"].(string); ok {
		result.ContentType = contentType
	}

	if contentLength, ok := finalResp.Headers["content-length"].(string); ok {
		if length, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			result.ContentLength = length
		}
	}

	// Parse server-timing header
	if serverTimingHeader, ok := finalResp.Headers["server-timing"].(string); ok {
		result.ServerTimings = parseServerTiming(serverTimingHeader)
	}

	// Calculate timing metrics using ResourceTiming exclusively
	// This ensures all measurements come from the browser's performance API
	if firstResp.Timing == nil {
		slog.Warn("First request has no timing data")
		return result
	}

	if finalResp.Timing == nil {
		slog.Warn("Final request has no timing data")
		return result
	}

	// Use ResourceTiming - all browser-measured, same clock source
	firstRequestTime := firstResp.Timing.RequestTime // seconds (baseline)
	finalRequestTime := finalResp.Timing.RequestTime // seconds (baseline)

	// Time from first request to final request start (in nanoseconds)
	result.TimeToFinalRedirect = time.Duration((finalRequestTime - firstRequestTime) * float64(time.Second))

	// TTFB within final request only (milliseconds offset from final request start)
	if finalResp.Timing.ReceiveHeadersStart > 0 {
		result.FinalTTFB = time.Duration(finalResp.Timing.ReceiveHeadersStart * float64(time.Millisecond)) // ms to ns
	}

	// Total TTFB from first request start to final headers received
	// = time to reach final request + TTFB within final request
	totalTTFBms := (finalRequestTime-firstRequestTime)*1e3 + finalResp.Timing.ReceiveHeadersStart
	result.TotalTTFB = time.Duration(totalTTFBms * float64(time.Millisecond)) // ms to ns

	return result
}

type HeliaConfig struct {
	TrustlessGateways            []string          `json:"gateways"`
	DelegatedRouters             []string          `json:"routers"`
	DNSJSONResolvers             map[string]string `json:"dnsJsonResolvers"`
	EnableRecursiveGateways      bool              `json:"enableRecursiveGateways"`
	EnableWss                    bool              `json:"enableWss"`
	EnableWebTransport           bool              `json:"enableWebTransport"`
	EnableGatewayProviders       bool              `json:"enableGatewayProviders"`
	Debug                        string            `json:"debug"`
	FetchTimeout                 int64             `json:"fetchTimeout"`
	ServiceWorkerRegistrationTTL int64             `json:"serviceWorkerRegistrationTTL"`
	AcceptOriginIsolationWarning bool              `json:"acceptOriginIsolationWarning"`
	SupportDirectoryIndexes      bool              `json:"supportDirectoryIndexes"`
	SupportWebRedirects          bool              `json:"supportWebRedirects"`
	RenderHTMLViews              bool              `json:"renderHTMLViews"`
}

func extractHeliaConfig(rawlURL string) *HeliaConfig {
	u, err := url.Parse(rawlURL)
	if err != nil {
		return nil
	}

	heliaConfigBase64Str := u.Query().Get("helia-config")
	if heliaConfigBase64Str == "" {
		return nil
	}
	heliaConfigStr, err := base64.RawStdEncoding.DecodeString(heliaConfigBase64Str)
	if err != nil {
		return nil
	}

	heliaConfig := &HeliaConfig{}
	if err := json.Unmarshal(heliaConfigStr, heliaConfig); err != nil {
		return nil
	}

	return heliaConfig
}

// handleRequestWillBeSent processes network request events for document types.
// It tracks document requests and updates trace information on redirects.
func (r *swProbe) handleRequestWillBeSent(e *network.EventRequestWillBeSent) {
	// Only track document requests
	if e.Type != network.ResourceTypeDocument {
		return
	}

	slog.Info("Sending request",
		"url", e.Request.URL,
		"isRedirect", e.RedirectResponse != nil,
	)

	// try to parse the helia config
	if heliaConfig := extractHeliaConfig(e.Request.URL); heliaConfig != nil {
		r.heliaConfig = heliaConfig
		for _, router := range heliaConfig.DelegatedRouters {
			u, err := url.Parse(router)
			if err != nil {
				continue
			}
			r.delegatedRouters[u.Host] = struct{}{}
		}

		for _, gateway := range heliaConfig.TrustlessGateways {
			u, err := url.Parse(gateway)
			if err != nil {
				continue
			}
			r.trustlessGateways[u.Host] = struct{}{}
		}
	}

	trace, found := r.documentRequests[e.RequestID]
	if !found {
		trace = &swRequestTrace{
			currentURL: e.DocumentURL,
			loaderID:   e.LoaderID,
		}

		r.documentRequestIDs = append(r.documentRequestIDs, e.RequestID)
		r.documentRequests[e.RequestID] = trace
	}

	if e.RedirectResponse == nil {
		return
	}

	slog.Debug("Redirecting...",
		"url", e.RedirectResponse.URL,
		"fromServiceWorker", e.RedirectResponse.FromServiceWorker,
		"status", e.RedirectResponse.Status,
	)

	trace.responses = append(trace.responses, e.RedirectResponse)
	trace.currentURL = e.RedirectResponse.URL
}

// handleResponseReceived processes document response events.
// Updates the current URL and appends the response to the trace.
func (r *swProbe) handleResponseReceived(e *network.EventResponseReceived) {
	// Only track document responses
	if e.Type != network.ResourceTypeDocument {
		return
	}

	slog.Info("Response received",
		"url", e.Response.URL,
		"fromServiceWorker", e.Response.FromServiceWorker,
		"status", e.Response.Status,
	)

	trace, found := r.documentRequests[e.RequestID]
	if !found {
		return
	}

	trace.currentURL = e.Response.URL
	trace.responses = append(trace.responses, e.Response)
}

type ServerTiming struct {
	Name  string
	Value time.Duration
	Desc  string
	Extra map[string]string
}

// parseServerTiming parses a server-timing header into a map of serverTiming.
// Each entry represents a metric with its name, duration, description, and extras.
// Parses strings like: custom-metric;dur=123.45;desc="My custom metric"
func parseServerTiming(raw string) map[string]ServerTiming {
	serverTimings := map[string]ServerTiming{}

	metrics := strings.Split(raw, ",")
	for _, metric := range metrics {
		fields := strings.Split(metric, ";")
		if len(fields) < 2 {
			continue
		}

		st := ServerTiming{
			Name:  strings.TrimSpace(fields[0]),
			Extra: make(map[string]string),
		}

		for _, field := range fields[1:] {
			kv := strings.Split(field, "=")
			if len(kv) != 2 {
				continue
			}

			switch kv[0] {
			case "dur":
				dur, err := time.ParseDuration(kv[1] + "ms")
				if err != nil {
					continue
				}
				st.Value = dur
			case "desc":
				unquote, err := strconv.Unquote(kv[1])
				if err != nil {
					continue
				}
				st.Desc = unquote
			default:
				unquote, err := strconv.Unquote(kv[1])
				if err != nil {
					continue
				}

				st.Extra[kv[0]] = unquote
			}
		}
		serverTimings[st.Name] = st
	}

	return serverTimings
}
