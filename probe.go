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

	"github.com/dennis-tra/tiros/models"
)

const websiteRequestTimeout = 15 * time.Second

var (
	ErrNavigateTimeout    = errors.New("navigation timed out")
	ErrOnLoadTimeout      = errors.New("window.onload event timed out")
	ErrNetworkIdleTimeout = errors.New("window.requestIdleCallback timed out")
)

type probeResult struct {
	url     string
	website string

	// measurement type (KUBO or HTTP)
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

	err error
}

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

		log.Infof("Letting Kubo settle for %s\n", sleepDur)
		time.Sleep(sleepDur)

		for i := 0; i < c.Int("times"); i++ {
			for _, mType := range []string{models.MeasurementTypeKUBO, models.MeasurementTypeHTTP} {
				for _, website := range websites {
					pr, err := t.probeWebsite(c, websiteURL(c, website, mType))
					if err != nil {
						log.WithError(err).WithField("website", website).Warnln("error probing website")
						continue
					}

					pr.website = website
					pr.mType = mType
					pr.try = i + j*len(c.IntSlice("settle-times"))

					log.WithFields(log.Fields{
						"ttfb": p2f(pr.ttfb),
						"lcp":  p2f(pr.lcp),
						"fcp":  p2f(pr.fcp),
						"tti":  p2f(pr.tti),
					}).WithError(pr.err).Infoln("Probed website", website)

					results <- pr

					if mType == models.MeasurementTypeKUBO {
						if err = t.KuboGC(c.Context); err != nil {
							log.WithError(err).Warnln("error running kubo gc")
							continue
						}
					}
				}
			}
		}
	}
}

func (t *tiros) probeWebsite(c *cli.Context, url string) (*probeResult, error) {
	logEntry := log.WithField("url", url)

	logEntry.Infoln("Probing", url)
	defer logEntry.Infoln("Done probing", url)

	pr := probeResult{
		url: url,
	}

	browser := rod.New().
		Context(c.Context). // stop when outer ctx stops
		ControlURL(fmt.Sprintf("ws://localhost:%d", c.Int("chrome-cdp-port")))

	logEntry.Debugln("Connecting to browser...")
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connecting to browser: %w", err)
	}
	defer func() {
		logEntry.Debugln("Closing connection to browser...")
		if err := browser.Close(); err != nil && !errors.Is(err, context.Canceled) {
			logEntry.WithError(err).Warnln("Error closing browser")
		}
	}()

	var metricsStr string
	var navPerfEntryStr string
	err := rod.Try(func() {
		logEntry.Debugln("Initialize incognito browser")
		browser = browser.Context(c.Context).MustIncognito() // first defense to prevent hitting the cache

		logEntry.Debugln("Clearing browser cookies") // empty arguments clear cookies
		browser.MustSetCookies()                     // second defense to prevent hitting the cache

		logEntry.Debugln("Opening new page")
		page := browser.MustPage()

		// clear local storage and attaching onerror listeners
		logEntry.Debugln("Attaching javascript")
		page.MustEvalOnNewDocument(jsOnNewDocument) // third defense to prevent hitting the cache

		// disable cache
		err := proto.NetworkSetCacheDisabled{CacheDisabled: true}.Call(page) // fourth defense to prevent hitting the cache
		if err != nil {
			panic(err)
		}

		logEntry.WithField("timeout", websiteRequestTimeout).Debugln("Navigating to", url, "...")
		err = page.Timeout(websiteRequestTimeout).Navigate(url)
		if errors.Is(err, context.DeadlineExceeded) {
			panic(ErrNavigateTimeout)
		} else if err != nil {
			panic(err)
		}

		logEntry.WithField("timeout", websiteRequestTimeout).Debugln("Waiting for onload event ...")
		// load: fired when the whole page has loaded (including all dependent resources such as stylesheets, scripts, iframes, and images)
		err = page.Timeout(websiteRequestTimeout).WaitLoad()
		if errors.Is(err, context.Canceled) {
			panic(err)
		} else if err != nil {
			pr.err = ErrOnLoadTimeout
		}

		logEntry.WithField("timeout", websiteRequestTimeout).Debugln("Waiting for network idle event ...")
		// idle: fired when network has come to a halt (1 Minute)
		err = page.Timeout(websiteRequestTimeout).WaitIdle(time.Minute)
		if errors.Is(err, context.Canceled) {
			panic(err)
		} else if err != nil {
			if pr.err != nil {
				err = fmt.Errorf("%s: %w", ErrNetworkIdleTimeout.Error(), pr.err)
			}
			pr.err = ErrNetworkIdleTimeout
		}

		logEntry.Debugln("Running polyfill JS ...")
		page.MustEval(wrapInFn(jsTTIPolyfill))   // add TTI polyfill
		page.MustEval(wrapInFn(jsWebVitalsIIFE)) // web-vitals

		logEntry.Debugln("Running measurement ...")
		metricsStr = page.MustEval(jsMeasurement).Str() // finally actually measure the stuff

		logEntry.Debugln("Getting Performance Entry measurement ...")
		rro, err := page.Eval(jsNavPerfEntry)
		if err != nil {
			logEntry.WithError(err).Warnln("Couldn't get navigation performance entry")
		} else {
			navPerfEntryStr = rro.Value.Str()
		}

		page.MustClose()
	})
	if errors.Is(err, ErrNavigateTimeout) {
		logEntry.WithError(err).Warnln("Couldn't measure website performance.")
		pr.err = ErrNavigateTimeout
		return &pr, nil
	} else if errors.Is(err, context.Canceled) {
		return nil, err
	} else if err != nil {
		logEntry.WithError(err).Warnln("Couldn't measure website performance.")
		pr.err = err
		return &pr, nil
	}

	vitals := WebVitals{}
	if err := json.Unmarshal([]byte(metricsStr), &vitals); err != nil {
		return nil, fmt.Errorf("unmarshal web-vitals: %w", err)
	}

	if navPerfEntryStr != "" {
		navPerfEntry := PerformanceNavigationEntry{}
		if err := json.Unmarshal([]byte(navPerfEntryStr), &navPerfEntry); err != nil {
			return nil, fmt.Errorf("unmarshal navigation performance entry: %w", err)
		}
		pr.navPerf = &navPerfEntry
	}

	for _, v := range vitals {
		v := v
		switch v.Name {
		case "LCP":
			pr.lcp = &v.Value
			pr.lcpRating = &v.Rating
		case "FCP":
			pr.fcp = &v.Value
			pr.fcpRating = &v.Rating
		case "TTFB":
			pr.ttfb = &v.Value
			pr.ttfbRating = &v.Rating
		case "TTI":
			pr.tti = &v.Value
			pr.ttiRating = &v.Rating
		case "CLS":
			pr.cls = &v.Value
			pr.clsRating = &v.Rating
		default:
			continue
		}
	}

	return &pr, nil
}

func websiteURL(c *cli.Context, website string, mType string) string {
	switch mType {
	case models.MeasurementTypeKUBO:
		return fmt.Sprintf("http://%s:%d/ipns/%s", c.String("kubo-host"), c.Int("kubo-gateway-port"), website)
	case models.MeasurementTypeHTTP:
		return fmt.Sprintf("https://%s", website)
	default:
		panic(fmt.Sprintf("unknown measurement type: %s", mType))
	}
}

func (t *tiros) KuboGC(ctx context.Context) error {
	return t.kubo.Request("repo/gc").Exec(ctx, nil)
}

func p2f(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
