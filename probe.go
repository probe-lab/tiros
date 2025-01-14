package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"github.com/volatiletech/null/v8"

	"github.com/probe-lab/tiros/models"
)

const websiteRequestTimeout = 15 * time.Second

var (
	ErrNavigateTimeout    = errors.New("navigation timed out")
	ErrOnLoadTimeout      = errors.New("window.onload event timed out")
	ErrNetworkIdleTimeout = errors.New("window.requestIdleCallback timed out")
)

func (pr *probeResult) NullError() null.String {
	if pr.err != nil {
		return null.StringFrom(pr.err.Error())
	}
	return null.NewString("", false)
}

func (t *tiros) measureWebsites(c *cli.Context, websites []string, results chan<- *probeResult) {
	defer close(results)

	for j, settle := range c.IntSlice("settle-times") {

		sleepDur := time.Duration(settle) * time.Second

		log.Infof("Letting the IPFS implementation settle for %s\n", sleepDur)
		time.Sleep(sleepDur)

		for i := 0; i < c.Int("times"); i++ {
			for _, mType := range []string{models.MeasurementTypeIPFS, models.MeasurementTypeHTTP} {
				for _, website := range websites {

					log.Infoln("Start probing", website, mType)
					pr, err := newProbe(c, website, mType).run(c)
					if errors.Is(c.Context.Err(), context.Canceled) {
						return
					} else if err != nil {
						log.WithError(err).WithField("website", website).Warnln("error probing website")
						continue
					}

					pr.website = website
					pr.mType = mType
					pr.try = i + j*len(c.IntSlice("settle-times"))

					log.WithFields(log.Fields{
						"ttfb":   p2f(pr.ttfb),
						"lcp":    p2f(pr.lcp),
						"fcp":    p2f(pr.fcp),
						"tti":    p2f(pr.tti),
						"status": pr.httpStatus,
					}).WithError(pr.err).Infoln("Probed website", website)

					results <- pr

					if mType == models.MeasurementTypeIPFS {
						if err = t.GarbageCollect(c.Context); err != nil {
							log.WithError(err).Warnln("error running ipfs gc")
							continue
						}
					}
				}
			}
		}
	}
}

type probe struct {
	ctx     context.Context
	url     string
	website string
	mType   string
	cdpPort int

	browser *rod.Browser
	page    *rod.Page

	result          *probeResult
	metricsStr      string
	navPerfEntryStr string
}

type probeResult struct {
	url     string
	website string

	// measurement type (IPFS or HTTP)
	mType string
	try   int

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
	httpBody   string

	err error
}

func newProbe(c *cli.Context, website string, mType string) *probe {
	return &probe{
		ctx:     c.Context,
		url:     websiteURL(c, website, mType),
		website: website,
		mType:   mType,
		cdpPort: c.Int("chrome-cdp-port"),
		result: &probeResult{
			url:     websiteURL(c, website, mType),
			website: website,
			mType:   mType,
		},
	}
}

func (p *probe) logEntry() *log.Entry {
	return log.WithField("url", p.url)
}

func (p *probe) run(c *cli.Context) (*probeResult, error) {
	if err := p.initBrowser(); err != nil {
		return nil, err
	}
	defer p.close()

	err := rod.Try(func() {
		p.mustInitPage()
		p.mustNavigate(c)
		p.mustMeasure()
	})

	if errors.Is(err, ErrNavigateTimeout) {
		p.logEntry().WithError(ErrNavigateTimeout).Warnln("Couldn't measure website performance.")
		p.result.err = ErrNavigateTimeout
		return p.result, nil
	} else if errors.Is(err, context.Canceled) {
		return nil, err
	} else if err != nil {
		p.logEntry().WithError(err).Warnln("Couldn't measure website performance.")
		p.result.err = err
		return p.result, nil
	}

	return p.result, p.parseMetrics()
}

func (p *probe) initBrowser() error {
	// Initialize browser reference
	p.browser = rod.New().
		Context(p.ctx). // stop when outer ctx stops
		ControlURL(fmt.Sprintf("ws://localhost:%d", p.cdpPort))

	// Connecting to headless chrome
	p.logEntry().Debugln("Connecting to browser...")
	if err := p.browser.Connect(); err != nil {
		return fmt.Errorf("connecting to browser: %w", err)
	}

	return nil
}

func (p *probe) mustInitPage() {
	p.logEntry().Debugln("Initialize incognito browser")
	// first defense to prevent hitting the cache.
	// use an incognito browser
	browser := p.browser.Context(p.ctx).MustIncognito()

	p.logEntry().Debugln("Clearing browser cookies")
	// second defense to prevent hitting the cache
	// empty arguments clear cookies
	browser.MustSetCookies()

	p.logEntry().Debugln("Opening new page")
	p.page = browser.MustPage()

	p.logEntry().Debugln("Attaching javascript")
	// third defense to prevent hitting the cache
	// clear local storage and attaching onerror listeners
	p.page.MustEvalOnNewDocument(jsOnNewDocument)

	// disable cache
	err := proto.NetworkSetCacheDisabled{CacheDisabled: true}.Call(p.page) // fourth defense to prevent hitting the cache
	if err != nil {
		panic(err)
	}
}

func (p *probe) mustNavigate(c *cli.Context) {
	if c.Bool("service-worker") {
		proto.ServiceWorkerEnable{}.Call(p.page)
	}
	e := proto.NetworkResponseReceived{}
	wait := p.page.Timeout(websiteRequestTimeout).WaitEvent(&e)

	p.logEntry().WithField("timeout", websiteRequestTimeout).Debugln("Navigating...")
	err := p.page.Timeout(websiteRequestTimeout).Navigate(p.url)

	wait()

	if e.Type == proto.NetworkResourceTypeDocument {
		p.result.httpStatus = e.Response.Status
		if p.result.httpStatus < 200 || p.result.httpStatus >= 300 {
			p.result.httpBody = p.page.MustElement("body").MustText()
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		panic(ErrNavigateTimeout)
	} else if err != nil {
		panic(err)
	}

	p.logEntry().WithField("timeout", websiteRequestTimeout).Debugln("Waiting for onload event ...")
	// load: fired when the whole page has loaded (including all dependent resources such as stylesheets, scripts, iframes, and images)
	err = p.page.Timeout(websiteRequestTimeout).WaitLoad()
	if errors.Is(err, context.Canceled) {
		panic(err)
	} else if err != nil {
		p.result.err = ErrOnLoadTimeout
	}

	p.logEntry().WithField("timeout", websiteRequestTimeout).Debugln("Waiting for network idle event ...")
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

	if c.Bool("service-worker") {
		// if there is no error, and idle was fired, log what the current url is
		p.logEntry().WithField("url", p.page.MustInfo().URL).Debugln("Network idle event fired")

		// log if a service worker is registered at the current URL
		p.checkServiceWorker()
	}
}

func (p *probe) checkServiceWorker() {
	p.logEntry().Debugln("Checking for service worker registration...")

	done := make(chan bool)
	// retryCount := 5
	// for i := 0; i < retryCount; i++ {
	go p.page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		logs := p.page.MustObjectsToJSON(e.Args)
		p.logEntry().Debugln(logs)
		for _, log := range logs.Arr() {
			if log.String() == "Service worker registered" {
				done <- true
				return
			}
		}

		close(done)
		// delay a bit to allow the service worker to register
	})

	p.page.MustEval(wrapInFn(jsCheckServiceWorkerRegistered))

	select {
	case registered := <-done:
		if registered {
			p.logEntry().Debugln("Service worker registered successfully")
			return
		}
	case <-time.After(websiteRequestTimeout):
		p.logEntry().Warnln("Service worker registration timed out, retrying...")
	}

	p.logEntry().Warnln("Service worker registration failed after retries")
}

func (p *probe) mustMeasure() {
	p.logEntry().Debugln("Running polyfill JS ...")
	p.page.MustEval(wrapInFn(jsTTIPolyfill))   // add TTI polyfill
	p.page.MustEval(wrapInFn(jsWebVitalsIIFE)) // web-vitals

	p.logEntry().Debugln("Running measurement ...")
	p.metricsStr = p.page.MustEval(jsMeasurement).Str() // finally measure the stuff

	p.logEntry().Debugln("Getting Performance Entry measurement ...")
	rro, err := p.page.Eval(jsNavPerfEntry)
	if err != nil {
		p.logEntry().WithError(err).Warnln("Couldn't get navigation performance entry")
	} else {
		p.navPerfEntryStr = rro.Value.Str()
	}
}

func (p *probe) parseMetrics() error {
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

func (p *probe) close() {
	p.logEntry().Debugln("Closing page...")
	if p.page != nil {
		if err := p.page.Close(); err != nil && !errors.Is(err, context.Canceled) {
			p.logEntry().WithError(err).Warnln("Error closing page")
		}
	}

	p.logEntry().Debugln("Closing browser connection...")
	if err := p.browser.Close(); err != nil && !errors.Is(err, context.Canceled) {
		p.logEntry().WithError(err).Warnln("Error closing browser connection")
	}
}

func websiteURL(c *cli.Context, website string, mType string) string {
	switch mType {
	case models.MeasurementTypeIPFS:
		return fmt.Sprintf("http://%s:%d/ipns/%s", c.String("ipfs-host"), c.Int("ipfs-gateway-port"), website)
	case models.MeasurementTypeHTTP:
		return fmt.Sprintf("https://%s", website)
	default:
		panic(fmt.Sprintf("unknown measurement type: %s", mType))
	}
}

func (t *tiros) GarbageCollect(ctx context.Context) error {
	return t.ipfs.Request("repo/gc").Exec(ctx, nil)
}

func p2f(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
