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
	"github.com/volatiletech/sqlboiler/v4/boil"

	"github.com/dennis-tra/tiros/models"
)

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
	log.Infoln("Probing", url)
	defer log.Infoln("Done probing", url)

	pr := ProbeResult{
		URL: url,
	}

	var perfEntriesStr string
	err := rod.Try(func() {
		// create new browser
		browser := rod.New().
			Context(c.Context). // stop when outer ctx stops
			ControlURL(fmt.Sprintf("ws://127.0.0.1:%d", c.Int("chrome-cdp-port"))).
			MustConnect().
			MustIncognito()

		// clear cookies
		browser.MustSetCookies()

		page := browser.MustPage()

		// clear local storage
		page.MustEvalOnNewDocument(`localStorage.clear();`)

		// disable cache
		err := proto.NetworkSetCacheDisabled{CacheDisabled: true}.Call(page)
		if err != nil {
			panic(err)
		}

		page.
			Timeout(websiteRequestTimeout). // only wait 30s
			MustNavigate(url).              // actually open the page
			MustWaitLoad().                 // fired when the whole page has loaded (ncluding all dependent resources such as stylesheets, scripts, iframes, and images)
			MustWaitIdle().                 // wait until network has come to a halt (1 Minute)
			CancelTimeout()                 // continue using page object for things

		perfEntriesStr = page.MustEval(jsPerformanceEntries).Str()

		page.MustClose()
		browser.MustClose()
	})
	if errors.Is(err, context.DeadlineExceeded) {
		pr.Error = fmt.Errorf("timed out after %s", websiteRequestTimeout)
		return &pr, nil
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

type ProbeResult struct {
	URL                    string
	TimeToFirstByte        *float64
	FirstContentfulPaint   *float64
	LargestContentfulPaint *float64
	NavigationPerformance  *PerformanceNavigationEntry
	Error                  error `json:"-"`
}

type Metrics struct {
	NavigationPerformance *PerformanceNavigationEntry
	// can grow
}

func (pr *ProbeResult) NullJSON() (null.JSON, error) {
	m := Metrics{
		NavigationPerformance: pr.NavigationPerformance,
	}

	data, err := json.Marshal(m)
	if err != nil {
		return null.NewJSON(nil, false), err
	}

	return null.JSONFrom(data), nil
}

func (pr *ProbeResult) NullError() null.String {
	if pr.Error != nil {
		return null.StringFrom(pr.Error.Error())
	}
	return null.NewString("", false)
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
