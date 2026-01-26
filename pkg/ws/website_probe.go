package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/probe-lab/go-commons/ptr"
	"github.com/probe-lab/tiros/pkg/db"
	"github.com/probe-lab/tiros/pkg/js"
)

const websiteRequestTimeout = 15 * time.Second

var (
	ErrNavigateTimeout    = errors.New("navigation timed out")
	ErrOnLoadTimeout      = errors.New("window.onload event timed out")
	ErrNetworkIdleTimeout = errors.New("window.requestIdleCallback timed out")
)

type WebsiteProbe struct {
	URL       string
	Website   string
	ProbeType db.WebsiteProbeProtocol
	CDPPort   int

	Browser *rod.Browser
	Page    *rod.Page

	Result          *WebsiteProbeResult
	MetricsStr      string
	NavPerfEntryStr string
}

type WebsiteProbeResult struct {
	URL     string
	Website string

	// measurement type (IPFS or HTTP)
	Protocol db.WebsiteProbeProtocol
	Try      int

	TTFB       *float64
	TTFBRating *string

	FCP       *float64
	FCPRating *string

	LCP       *float64
	LCPRating *string

	TTI       *float64
	TTIRating *string

	CLS       *float64
	CLSRating *string

	NavPerf *PerformanceNavigationEntry

	HTTPStatus int
	HTTPBody   *string

	Err error
}

func (p *WebsiteProbe) logEntry() *slog.Logger {
	return slog.With("url", p.URL)
}

func (p *WebsiteProbe) Run(ctx context.Context) (*WebsiteProbeResult, error) {
	if err := p.initBrowser(ctx); err != nil {
		return nil, err
	}
	defer p.close()

	err := rod.Try(func() {
		p.mustInitPage(ctx)
		p.mustNavigate()
		p.mustMeasure()
	})

	if errors.Is(err, ErrNavigateTimeout) {
		p.logEntry().With("err", ErrNavigateTimeout).Warn("Couldn't measure website performance.")
		p.Result.Err = ErrNavigateTimeout
		return p.Result, nil
	} else if errors.Is(err, context.Canceled) {
		return nil, err
	} else if err != nil {
		p.logEntry().With("err", err).Warn("Couldn't measure website performance.")
		p.Result.Err = err
		return p.Result, nil
	}

	return p.Result, p.parseMetrics()
}

func (p *WebsiteProbe) initBrowser(ctx context.Context) error {
	// Initialize browser reference
	p.Browser = rod.New().
		Context(ctx). // stop when outer ctx stops
		ControlURL(fmt.Sprintf("ws://localhost:%d", p.CDPPort))

	// Connecting to headless chrome
	p.logEntry().Debug("Connecting to browser...")
	if err := p.Browser.Connect(); err != nil {
		return fmt.Errorf("connecting to browser: %w", err)
	}

	return nil
}

func (p *WebsiteProbe) mustInitPage(ctx context.Context) {
	p.logEntry().Debug("Initialize incognito browser")
	// first defense to prevent hitting the cache.
	// use an incognito browser
	browser := p.Browser.Context(ctx).MustIncognito()

	p.logEntry().Debug("Clearing browser cookies")
	// second defense to prevent hitting the cache
	// empty arguments clear cookies
	browser.MustSetCookies()

	p.logEntry().Debug("Opening new page")
	p.Page = browser.MustPage()

	p.logEntry().Debug("Attaching javascript")
	// third defense to prevent hitting the cache
	// clear local storage and attaching onerror listeners
	p.Page.MustEvalOnNewDocument(js.OnNewDocument)

	// disable cache
	err := proto.NetworkSetCacheDisabled{CacheDisabled: true}.Call(p.Page) // fourth defense to prevent hitting the cache
	if err != nil {
		panic(err)
	}
}

func (p *WebsiteProbe) mustNavigate() {
	e := proto.NetworkResponseReceived{}
	wait := p.Page.Timeout(websiteRequestTimeout).WaitEvent(&e)

	p.logEntry().With("timeout", websiteRequestTimeout).Debug("Navigating...")
	err := p.Page.Timeout(websiteRequestTimeout).Navigate(p.URL)

	wait()

	if e.Type == proto.NetworkResourceTypeDocument {
		p.Result.HTTPStatus = e.Response.Status
		if p.Result.HTTPStatus < 200 || p.Result.HTTPStatus >= 300 {
			p.Result.HTTPBody = ptr.From(p.Page.MustElement("body").MustText())
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		panic(ErrNavigateTimeout)
	} else if err != nil {
		panic(err)
	}

	p.logEntry().With("timeout", websiteRequestTimeout).Debug("Waiting for onload event ...")
	// load: fired when the whole page has loaded (including all dependent resources such as stylesheets, scripts, iframes, and images)
	err = p.Page.Timeout(websiteRequestTimeout).WaitLoad()
	if errors.Is(err, context.Canceled) {
		panic(err)
	} else if err != nil {
		p.Result.Err = ErrOnLoadTimeout
	}

	p.logEntry().With("timeout", websiteRequestTimeout).Debug("Waiting for network idle event ...")
	// idle: fired when the network has come to a halt (1 Minute)
	err = p.Page.Timeout(websiteRequestTimeout).WaitIdle(time.Minute)
	if errors.Is(err, context.Canceled) {
		panic(err)
	} else if err != nil {
		if p.Result.Err != nil {
			err = fmt.Errorf("%s: %w", ErrNetworkIdleTimeout.Error(), p.Result.Err)
		}
		p.Result.Err = ErrNetworkIdleTimeout
	}
}

func (p *WebsiteProbe) mustMeasure() {
	p.logEntry().Debug("Running polyfill JS ...")
	p.Page.MustEval(js.WrapInFn(js.TTIPolyfill))   // add TTI polyfill
	p.Page.MustEval(js.WrapInFn(js.WebVitalsIIFE)) // web-vitals

	p.logEntry().Debug("Running measurement ...")
	p.MetricsStr = p.Page.MustEval(js.Measurement).Str() // finally measure the stuff

	p.logEntry().Debug("Getting Performance Entry measurement ...")
	rro, err := p.Page.Eval(js.NavPerfEntry)
	if err != nil {
		p.logEntry().With("err", err).Warn("Couldn't get navigation performance entry")
	} else {
		p.NavPerfEntryStr = rro.Value.Str()
	}
}

func (p *WebsiteProbe) parseMetrics() error {
	vitals := js.WebVitals{}
	if err := json.Unmarshal([]byte(p.MetricsStr), &vitals); err != nil {
		return fmt.Errorf("unmarshal web-vitals: %w", err)
	}

	if p.NavPerfEntryStr != "" {
		navPerfEntry := PerformanceNavigationEntry{}
		if err := json.Unmarshal([]byte(p.NavPerfEntryStr), &navPerfEntry); err != nil {
			return fmt.Errorf("unmarshal navigation performance entry: %w", err)
		}
		p.Result.NavPerf = &navPerfEntry
	}

	for _, v := range vitals {
		v := v
		switch v.Name {
		case "LCP":
			p.Result.LCP = &v.Value
			p.Result.LCPRating = &v.Rating
		case "FCP":
			p.Result.FCP = &v.Value
			p.Result.FCPRating = &v.Rating
		case "TTFB":
			p.Result.TTFB = &v.Value
			p.Result.TTFBRating = &v.Rating
		case "TTI":
			p.Result.TTI = &v.Value
			p.Result.TTIRating = &v.Rating
		case "CLS":
			p.Result.CLS = &v.Value
			p.Result.CLSRating = &v.Rating
		default:
			continue
		}
	}

	return nil
}

func (p *WebsiteProbe) close() {
	p.logEntry().Debug("Closing page...")
	if p.Page != nil {
		if err := p.Page.Close(); err != nil && !errors.Is(err, context.Canceled) {
			p.logEntry().With("err", err).Warn("Error closing page")
		}
	}

	p.logEntry().Debug("Closing browser connection...")
	if err := p.Browser.Close(); err != nil && !errors.Is(err, context.Canceled) {
		p.logEntry().With("err", err).Warn("Error closing browser connection")
	}
}

func p2f(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
