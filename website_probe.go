package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const websiteRequestTimeout = 15 * time.Second

var (
	ErrNavigateTimeout    = errors.New("navigation timed out")
	ErrOnLoadTimeout      = errors.New("window.onload event timed out")
	ErrNetworkIdleTimeout = errors.New("window.requestIdleCallback timed out")
)

type websiteProbe struct {
	url       string
	website   string
	probeType WebsiteProbeProtocol
	cdpPort   int

	browser *rod.Browser
	page    *rod.Page

	result          *websiteProbeResult
	metricsStr      string
	navPerfEntryStr string
}

type websiteProbeResult struct {
	url     string
	website string

	// measurement type (IPFS or HTTP)
	protocol WebsiteProbeProtocol
	try      int

	ttfb       *float64
	ttfbRating *string

	fcp       *float64
	fcpRating *string

	lcp       *float64
	lcpRating *string

	tti       *float64
	ttiRating *string

	cls       *float64
	clsRating *string

	navPerf *PerformanceNavigationEntry

	httpStatus int
	httpBody   *string

	err error
}

func (p *websiteProbe) logEntry() *slog.Logger {
	return slog.With("url", p.url)
}

func (p *websiteProbe) run(ctx context.Context) (*websiteProbeResult, error) {
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
		p.result.err = ErrNavigateTimeout
		return p.result, nil
	} else if errors.Is(err, context.Canceled) {
		return nil, err
	} else if err != nil {
		p.logEntry().With("err", err).Warn("Couldn't measure website performance.")
		p.result.err = err
		return p.result, nil
	}

	return p.result, p.parseMetrics()
}

func (p *websiteProbe) initBrowser(ctx context.Context) error {
	// Initialize browser reference
	p.browser = rod.New().
		Context(ctx). // stop when outer ctx stops
		ControlURL(fmt.Sprintf("ws://localhost:%d", p.cdpPort))

	// Connecting to headless chrome
	p.logEntry().Debug("Connecting to browser...")
	if err := p.browser.Connect(); err != nil {
		return fmt.Errorf("connecting to browser: %w", err)
	}

	return nil
}

func (p *websiteProbe) mustInitPage(ctx context.Context) {
	p.logEntry().Debug("Initialize incognito browser")
	// first defense to prevent hitting the cache.
	// use an incognito browser
	browser := p.browser.Context(ctx).MustIncognito()

	p.logEntry().Debug("Clearing browser cookies")
	// second defense to prevent hitting the cache
	// empty arguments clear cookies
	browser.MustSetCookies()

	p.logEntry().Debug("Opening new page")
	p.page = browser.MustPage()

	p.logEntry().Debug("Attaching javascript")
	// third defense to prevent hitting the cache
	// clear local storage and attaching onerror listeners
	p.page.MustEvalOnNewDocument(jsOnNewDocument)

	// disable cache
	err := proto.NetworkSetCacheDisabled{CacheDisabled: true}.Call(p.page) // fourth defense to prevent hitting the cache
	if err != nil {
		panic(err)
	}
}

func (p *websiteProbe) mustNavigate() {
	e := proto.NetworkResponseReceived{}
	wait := p.page.Timeout(websiteRequestTimeout).WaitEvent(&e)

	p.logEntry().With("timeout", websiteRequestTimeout).Debug("Navigating...")
	err := p.page.Timeout(websiteRequestTimeout).Navigate(p.url)

	wait()

	if e.Type == proto.NetworkResourceTypeDocument {
		p.result.httpStatus = e.Response.Status
		if p.result.httpStatus < 200 || p.result.httpStatus >= 300 {
			p.result.httpBody = toPtr(p.page.MustElement("body").MustText())
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		panic(ErrNavigateTimeout)
	} else if err != nil {
		panic(err)
	}

	p.logEntry().With("timeout", websiteRequestTimeout).Debug("Waiting for onload event ...")
	// load: fired when the whole page has loaded (including all dependent resources such as stylesheets, scripts, iframes, and images)
	err = p.page.Timeout(websiteRequestTimeout).WaitLoad()
	if errors.Is(err, context.Canceled) {
		panic(err)
	} else if err != nil {
		p.result.err = ErrOnLoadTimeout
	}

	p.logEntry().With("timeout", websiteRequestTimeout).Debug("Waiting for network idle event ...")
	// idle: fired when the network has come to a halt (1 Minute)
	err = p.page.Timeout(websiteRequestTimeout).WaitIdle(time.Minute)
	if errors.Is(err, context.Canceled) {
		panic(err)
	} else if err != nil {
		if p.result.err != nil {
			err = fmt.Errorf("%s: %w", ErrNetworkIdleTimeout.Error(), p.result.err)
		}
		p.result.err = ErrNetworkIdleTimeout
	}
}

func (p *websiteProbe) mustMeasure() {
	p.logEntry().Debug("Running polyfill JS ...")
	p.page.MustEval(wrapInFn(jsTTIPolyfill))   // add TTI polyfill
	p.page.MustEval(wrapInFn(jsWebVitalsIIFE)) // web-vitals

	p.logEntry().Debug("Running measurement ...")
	p.metricsStr = p.page.MustEval(jsMeasurement).Str() // finally measure the stuff

	p.logEntry().Debug("Getting Performance Entry measurement ...")
	rro, err := p.page.Eval(jsNavPerfEntry)
	if err != nil {
		p.logEntry().With("err", err).Warn("Couldn't get navigation performance entry")
	} else {
		p.navPerfEntryStr = rro.Value.Str()
	}
}

func (p *websiteProbe) parseMetrics() error {
	vitals := WebVitals{}
	if err := json.Unmarshal([]byte(p.metricsStr), &vitals); err != nil {
		return fmt.Errorf("unmarshal web-vitals: %w", err)
	}

	if p.navPerfEntryStr != "" {
		navPerfEntry := PerformanceNavigationEntry{}
		if err := json.Unmarshal([]byte(p.navPerfEntryStr), &navPerfEntry); err != nil {
			return fmt.Errorf("unmarshal navigation performance entry: %w", err)
		}
		p.result.navPerf = &navPerfEntry
	}

	for _, v := range vitals {
		v := v
		switch v.Name {
		case "LCP":
			p.result.lcp = &v.Value
			p.result.lcpRating = &v.Rating
		case "FCP":
			p.result.fcp = &v.Value
			p.result.fcpRating = &v.Rating
		case "TTFB":
			p.result.ttfb = &v.Value
			p.result.ttfbRating = &v.Rating
		case "TTI":
			p.result.tti = &v.Value
			p.result.ttiRating = &v.Rating
		case "CLS":
			p.result.cls = &v.Value
			p.result.clsRating = &v.Rating
		default:
			continue
		}
	}

	return nil
}

func (p *websiteProbe) close() {
	p.logEntry().Debug("Closing page...")
	if p.page != nil {
		if err := p.page.Close(); err != nil && !errors.Is(err, context.Canceled) {
			p.logEntry().With("err", err).Warn("Error closing page")
		}
	}

	p.logEntry().Debug("Closing browser connection...")
	if err := p.browser.Close(); err != nil && !errors.Is(err, context.Canceled) {
		p.logEntry().With("err", err).Warn("Error closing browser connection")
	}
}

func p2f(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
