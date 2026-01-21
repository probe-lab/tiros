package main

import (
	"context"
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
	"github.com/chromedp/chromedp"
)

type swProbe struct {
	url     string
	cdpHost string
	cdpPort int

	listenMu sync.Mutex

	// document requests
	requestIDs []network.RequestID
	requests   map[network.RequestID]*swRequestTrace

	navigationEvents []*page.EventFrameNavigated
	lifecycleEvents  map[cdp.LoaderID][]*page.EventLifecycleEvent
}

func newSwProbe(url string, cdpHost string, cdpPort int) *swProbe {
	return &swProbe{
		url:     url,
		cdpHost: cdpHost,
		cdpPort: cdpPort,

		requestIDs:       make([]network.RequestID, 0),
		requests:         make(map[network.RequestID]*swRequestTrace),
		navigationEvents: make([]*page.EventFrameNavigated, 0),
		lifecycleEvents:  map[cdp.LoaderID][]*page.EventLifecycleEvent{},
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

	// Server timing data (from final response)
	ServerTimings map[string]serverTiming
}

func (r *swProbe) isProbeDone() bool {
	// first check if we have seen the final request
	// for the content from the service worker gateway
	var finalRequest *swRequestTrace
	for _, req := range r.requests {
		if req.isFinalRequest() {
			finalRequest = req
			break
		}
	}

	if finalRequest == nil {
		return false
	}

	// Then wait until the network is idle in the final navigation
	lifecycleEvents, found := r.lifecycleEvents[finalRequest.loaderID]
	if !found {
		return false
	}

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

// finalResponse returns the last response in the trace or nil if no responses exist.
func (r *swRequestTrace) finalResponse() *network.Response {
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
	finalResp := r.finalResponse()
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

	_, constainsXIPFSPathHeader := finalResp.Headers[xIPFSPathHeader]
	_, constainsXIPFSRootsHeader := finalResp.Headers[xIPFSRootsHeader]

	if containsAccessControlExposeHeadersValues || constainsXIPFSPathHeader || constainsXIPFSRootsHeader {
		return true
	}

	return false
}

func (p *swProbe) run(ctx context.Context) (*swProbeResult, error) {
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
		chromedp.Navigate(p.url),
	)
	if err != nil {
		return nil, fmt.Errorf("enabling domains and navigation: %w", err)
	}

	// Wait for completion or timeout
	<-listenCtx.Done()

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

// buildProbeResult constructs the complete probe result from collected data
func (p *swProbe) buildProbeResult() *swProbeResult {
	result := &swProbeResult{
		ServerTimings: make(map[string]serverTiming),
	}

	// Find first and final requests
	var firstReq *swRequestTrace
	var finalReq *swRequestTrace

	for _, reqID := range p.requestIDs {
		req := p.requests[reqID]
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
	finalResp := finalReq.finalResponse()

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

// handleFrameNavigated processes page navigation events
func (r *swProbe) handleFrameNavigated(e *page.EventFrameNavigated) {
	slog.Debug("Navigated to "+e.Frame.URL, "loaderID", e.Frame.LoaderID)
	r.navigationEvents = append(r.navigationEvents, e)
}

// handleLifecycleEvent processes page lifecycle events
func (r *swProbe) handleLifecycleEvent(e *page.EventLifecycleEvent) {
	slog.Debug("Lifecycle event",
		"name", e.Name,
		"loader", e.LoaderID,
		"frame", e.FrameID,
	)

	events, found := r.lifecycleEvents[e.LoaderID]
	if !found {
		events = make([]*page.EventLifecycleEvent, 0)
		r.lifecycleEvents[e.LoaderID] = events
	}

	r.lifecycleEvents[e.LoaderID] = append(events, e)
}

// handleRequestWillBeSent processes network request events for document types.
// It tracks document requests and updates trace information on redirects.
func (r *swProbe) handleRequestWillBeSent(e *network.EventRequestWillBeSent) {
	// Only track document requests
	if e.Type != network.ResourceTypeDocument {
		return
	}

	slog.Info("Requesting",
		"url", e.Request.URL,
		"isRedirect", e.RedirectResponse != nil,
	)

	trace, found := r.requests[e.RequestID]
	if !found {
		trace = &swRequestTrace{
			currentURL: e.DocumentURL,
			loaderID:   e.LoaderID,
		}

		r.requestIDs = append(r.requestIDs, e.RequestID)
		r.requests[e.RequestID] = trace
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

	trace, found := r.requests[e.RequestID]
	if !found {
		return
	}

	trace.currentURL = e.Response.URL
	trace.responses = append(trace.responses, e.Response)
}

type serverTiming struct {
	name  string
	value time.Duration
	desc  string
	extra map[string]string
}

// parseServerTiming parses a server-timing header into a map of serverTiming.
// Each entry represents a metric with its name, duration, description, and extras.
// Parses strings like: custom-metric;dur=123.45;desc="My custom metric"
func parseServerTiming(raw string) map[string]serverTiming {
	serverTimings := map[string]serverTiming{}

	metrics := strings.Split(raw, ",")
	for _, metric := range metrics {
		fields := strings.Split(metric, ";")
		if len(fields) < 2 {
			continue
		}

		st := serverTiming{
			name:  strings.TrimSpace(fields[0]),
			extra: make(map[string]string),
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
				st.value = dur
			case "desc":
				unquote, err := strconv.Unquote(kv[1])
				if err != nil {
					continue
				}
				st.desc = unquote
			default:
				unquote, err := strconv.Unquote(kv[1])
				if err != nil {
					continue
				}

				st.extra[kv[0]] = unquote
			}
		}
		serverTimings[st.name] = st
	}

	return serverTimings
}
