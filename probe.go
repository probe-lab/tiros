package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	shell "github.com/ipfs/go-ipfs-api"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"

	"github.com/dennis-tra/tiros/models"
)

var (
	ErrNavigateTimeout    = errors.New("navigation timed out")
	ErrOnLoadTimeout      = errors.New("window.onload event timed out")
	ErrNetworkIdleTimeout = errors.New("window.requestIdleCallback timed out")
)

func handleTimeoutErr(err error, deadlineErr error) {
	if errors.Is(err, context.DeadlineExceeded) {
		panic(deadlineErr)
	} else if err != nil {
		panic(err)
	}
}

const websiteRequestTimeout = 30 * time.Second

type Tiros struct {
	DBClient *DBClient
	Kubo     *shell.Shell
	DBRun    *models.Run
}

func (t *Tiros) InitRun(c *cli.Context) (*models.Run, error) {
	version, sha, err := t.Kubo.Version()
	if err != nil {
		return nil, fmt.Errorf("kubo api offline: %w", err)
	}

	dbRun, err := t.DBClient.InsertRun(c, fmt.Sprintf("%s-%s", version, sha))
	if err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}

	t.DBRun = dbRun

	return t.DBRun, nil
}

func (t *Tiros) Probe(c *cli.Context, url string) (*ProbeResult, error) {
	logEntry := log.WithField("url", url)

	logEntry.Infoln("Probing", url)
	defer logEntry.Infoln("Done probing", url)

	pr := ProbeResult{
		URL: url,
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
		if err := browser.Close(); err != nil {
			logEntry.WithError(err).Warnln("Error closing browser")
		}
	}()

	var perfEntriesStr string
	err := rod.Try(func() {
		logEntry.Debugln("Initialize incognito browser")
		browser = browser.MustIncognito() // first defense to prevent hitting the cache

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

		logEntry.WithField("timeout", websiteRequestTimeout).Debugln("Navigating to", url, "and waiting for page to settle...")
		// load: fired when the whole page has loaded (including all dependent resources such as stylesheets, scripts, iframes, and images)
		// idle: fired when network has come to a halt (1 Minute)
		err = page.Timeout(websiteRequestTimeout).Navigate(url)
		handleTimeoutErr(err, ErrNavigateTimeout)

		err = page.Timeout(websiteRequestTimeout).WaitLoad()
		handleTimeoutErr(err, ErrOnLoadTimeout)

		err = page.Timeout(websiteRequestTimeout).WaitIdle(time.Minute)
		// don't error out but just keep track of it
		if err != nil {
			pr.Error = ErrNetworkIdleTimeout
		}

		logEntry.WithFields(log.Fields{
			"href":       page.MustEval("() => window.location.href").Str(),
			"errorCount": page.MustEval("() => `${window.errorCount}`").Str(),
		}).Debugln("Getting performance entries...")
		perfEntriesStr = page.MustEval(jsPerformanceEntries).Str()

		page.MustClose()
	})
	if err != nil {
		logEntry.WithError(err).Warnln("Couldn't measure website performance.")
	}
	if errors.Is(err, ErrNavigateTimeout) {
		pr.Error = ErrNavigateTimeout
		return &pr, nil
	} else if errors.Is(err, ErrOnLoadTimeout) {
		pr.Error = ErrOnLoadTimeout
		return &pr, nil
		//} else if errors.Is(err, ErrNetworkIdleTimeout) {
		//	pr.Error = ErrNetworkIdleTimeout
		//	return &pr, nil
	} else if errors.Is(err, context.Canceled) {
		return nil, err
	} else if err != nil {
		pr.Error = err
		return &pr, nil
	}

	perfEntries, err := unmarshalPerformanceEntries([]byte(perfEntriesStr))
	if err != nil {
		fmt.Println(perfEntriesStr)
		return nil, fmt.Errorf("parse performance entries: %w", err)
	}

	// https://developer.mozilla.org/en-US/docs/Learn/Performance/Perceived_performance#performance_metrics
	for _, e := range perfEntries {
		switch e.EntryType {
		case "resource":
		case "navigation":
			pne, err := e.NavigationEntry()
			if err != nil {
				return nil, fmt.Errorf("parse navigation entry: %w", err)
			}

			// https://developer.mozilla.org/en-US/docs/Web/Performance/Navigation_and_resource_timings#time_to_first_byte
			ttfb := pne.ResponseStart - pne.StartTime
			pr.TimeToFirstByte = &ttfb
			pr.NavigationPerformance = pne

		case "paint":
			if e.Name != "first-contentful-paint" {
				continue
			}

			// https://web.dev/fcp/
			if pr.FirstContentfulPaint == nil || e.StartTime < *pr.FirstContentfulPaint {
				pr.FirstContentfulPaint = &e.StartTime
			}

		case "largest-contentful-paint":
			// https://web.dev/lcp/
			if pr.LargestContentfulPaint == nil || e.StartTime > *pr.LargestContentfulPaint {
				pr.LargestContentfulPaint = &e.StartTime
			}
		}
	}

	return &pr, nil
}

func (t *Tiros) SealRun(ctx context.Context) (*models.Run, error) {
	t.DBRun.FinishedAt = null.TimeFrom(time.Now())
	_, err := t.DBRun.Update(ctx, t.DBClient.handle, boil.Infer())
	return t.DBRun, err
}

func (t *Tiros) Save(c *cli.Context, pr *ProbeResult, website string, mType string, try int) (*models.Measurement, error) {
	metrics, err := pr.NullJSON()
	if err != nil {
		return nil, fmt.Errorf("extract metrics from probe result: %w", err)
	}

	m := &models.Measurement{
		RunID:   t.DBRun.ID,
		Website: website,
		URL:     pr.URL,
		Type:    mType,
		Try:     int16(try),
		TTFB:    intervalMs(pr.TimeToFirstByte),
		FCP:     intervalMs(pr.FirstContentfulPaint),
		LCP:     intervalMs(pr.LargestContentfulPaint),
		Metrics: metrics,
		Error:   pr.NullError(),
	}

	if err := m.Insert(c.Context, t.DBClient.handle, boil.Infer()); err != nil {
		return nil, fmt.Errorf("insert measurement: %w", err)
	}

	return m, nil
}

func (t *Tiros) KuboGC(ctx context.Context) error {
	return t.Kubo.Request("repo/gc").Exec(ctx, nil)
}

func intervalMs(val *float64) null.String {
	if val == nil {
		return null.NewString("", false)
	}
	return null.StringFrom(fmt.Sprintf("%f Milliseconds", *val))
}
