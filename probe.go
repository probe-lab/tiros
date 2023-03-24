package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	shell "github.com/ipfs/go-ipfs-api"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"github.com/volatiletech/null/v8"

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
	DBClient IDBClient
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
		if err := browser.Close(); err != nil && !errors.Is(err, context.Canceled) {
			logEntry.WithError(err).Warnln("Error closing browser")
		}
	}()

	var metricsStr string
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
		handleTimeoutErr(err, ErrNavigateTimeout)

		logEntry.WithField("timeout", websiteRequestTimeout).Debugln("Waiting for onload event ...")
		// load: fired when the whole page has loaded (including all dependent resources such as stylesheets, scripts, iframes, and images)
		err = page.Timeout(websiteRequestTimeout).WaitLoad()
		if errors.Is(err, context.Canceled) {
			panic(err)
		} else if err != nil {
			pr.Error = ErrOnLoadTimeout
		}

		logEntry.WithField("timeout", websiteRequestTimeout).Debugln("Waiting for network idle event ...")
		// idle: fired when network has come to a halt (1 Minute)
		err = page.Timeout(websiteRequestTimeout).WaitIdle(time.Minute)
		if errors.Is(err, context.Canceled) {
			panic(err)
		} else if err != nil {
			if pr.Error != nil {
				err = fmt.Errorf("%s: %w", ErrNetworkIdleTimeout.Error(), pr.Error)
			}
			pr.Error = ErrNetworkIdleTimeout
		}

		logEntry.Debugln("Running polyfill JS ...")
		page.MustEval(wrapInFn(jsTTIPolyfill)) // add TTI polyfill + web-vitals
		page.MustEval(wrapInFn(jsWebVitalsIIFE))

		logEntry.Debugln("Running measurement ...")
		metricsStr = page.MustEval(jsMeasurement).Str() // finally actually measure the stuff

		page.MustClose()
	})
	if errors.Is(err, ErrNavigateTimeout) {
		logEntry.WithError(err).Warnln("Couldn't measure website performance.")
		pr.Error = ErrNavigateTimeout
		return &pr, nil
	} else if errors.Is(err, context.Canceled) {
		return nil, err
	} else if err != nil {
		logEntry.WithError(err).Warnln("Couldn't measure website performance.")
		pr.Error = err
		return &pr, nil
	}

	vitals := WebVitals{}
	if err := json.Unmarshal([]byte(metricsStr), &vitals); err != nil {
		return nil, fmt.Errorf("unmarshal web-vitals: %w", err)
	}

	for _, v := range vitals {
		v := v
		switch v.Name {
		case "LCP":
			pr.LargestContentfulPaint = &v.Value
			pr.LargestContentfulPaintRating = &v.Rating
		case "FCP":
			pr.FirstContentfulPaint = &v.Value
			pr.FirstContentfulPaintRating = &v.Rating
		case "TTFB":
			pr.TimeToFirstByte = &v.Value
			pr.TimeToFirstByteRating = &v.Rating
		case "TTI":
			pr.TimeToInteract = &v.Value
			pr.TimeToInteractRating = &v.Rating
		case "CLS":
			pr.CumulativeLayoutShift = &v.Value
			pr.CumulativeLayoutShiftRating = &v.Rating
		}
		logEntry.Infoln(v.Name, v.Value, v.Rating)
	}

	return &pr, nil
}

type ProbeResult struct {
	URL string

	TimeToFirstByte       *float64
	TimeToFirstByteRating *string

	FirstContentfulPaint       *float64
	FirstContentfulPaintRating *string

	LargestContentfulPaint       *float64
	LargestContentfulPaintRating *string

	TimeToInteract       *float64
	TimeToInteractRating *string

	CumulativeLayoutShift       *float64
	CumulativeLayoutShiftRating *string

	Error error `json:"-"`
}

func (pr *ProbeResult) NullError() null.String {
	if pr.Error != nil {
		return null.StringFrom(pr.Error.Error())
	}
	return null.NewString("", false)
}

func (t *Tiros) KuboGC(ctx context.Context) error {
	return t.Kubo.Request("repo/gc").Exec(ctx, nil)
}
