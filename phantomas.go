package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	kubo "github.com/guseggert/clustertest-kubo"
	"github.com/guseggert/clustertest/cluster"
)

type Metrics struct {
	Requests                       int64       `json:"requests"`
	GzipRequests                   int64       `json:"gzipRequests"`
	PostRequests                   int64       `json:"postRequests"`
	HTTPSRequests                  int64       `json:"httpsRequests"`
	NotFound                       int64       `json:"notFound"`
	BodySize                       int64       `json:"bodySize"`
	ContentLength                  int64       `json:"contentLength"`
	HTTPTrafficCompleted           *float64    `json:"httpTrafficCompleted"`
	TimeToFirstByte                float64     `json:"timeToFirstByte"`
	TimeToLastByte                 float64     `json:"timeToLastByte"`
	AjaxRequests                   int64       `json:"ajaxRequests"`
	SynchronousXHR                 int64       `json:"synchronousXHR"`
	WindowAlerts                   int64       `json:"windowAlerts"`
	WindowConfirms                 int64       `json:"windowConfirms"`
	WindowPrompts                  int64       `json:"windowPrompts"`
	HTMLCount                      int64       `json:"htmlCount"`
	HTMLSize                       int64       `json:"htmlSize"`
	CSSCount                       int64       `json:"cssCount"`
	CSSSize                        int64       `json:"cssSize"`
	JSCount                        int64       `json:"jsCount"`
	JSSize                         int64       `json:"jsSize"`
	JSONCount                      int64       `json:"jsonCount"`
	JSONSize                       int64       `json:"jsonSize"`
	ImageCount                     int64       `json:"imageCount"`
	ImageSize                      int64       `json:"imageSize"`
	VideoCount                     int64       `json:"videoCount"`
	VideoSize                      int64       `json:"videoSize"`
	WebfontCount                   int64       `json:"webfontCount"`
	WebfontSize                    int64       `json:"webfontSize"`
	Base64Count                    int64       `json:"base64Count"`
	Base64Size                     int64       `json:"base64Size"`
	OtherCount                     int64       `json:"otherCount"`
	OtherSize                      int64       `json:"otherSize"`
	BlockedRequests                int64       `json:"blockedRequests"`
	CacheHits                      int64       `json:"cacheHits"`
	CacheMisses                    int64       `json:"cacheMisses"`
	CachePasses                    int64       `json:"cachePasses"`
	CachingNotSpecified            int64       `json:"cachingNotSpecified"`
	CachingTooShort                int64       `json:"cachingTooShort"`
	CachingDisabled                int64       `json:"cachingDisabled"`
	OldCachingHeaders              int64       `json:"oldCachingHeaders"`
	CachingUseImmutable            int64       `json:"cachingUseImmutable"`
	ConsoleMessages                int64       `json:"consoleMessages"`
	CookiesSent                    int64       `json:"cookiesSent"`
	CookiesRecv                    int64       `json:"cookiesRecv"`
	DomainsWithCookies             int64       `json:"domainsWithCookies"`
	DocumentCookiesLength          int64       `json:"documentCookiesLength"`
	DocumentCookiesCount           int64       `json:"documentCookiesCount"`
	DocumentHeight                 int64       `json:"documentHeight"`
	BodyHTMLSize                   int64       `json:"bodyHTMLSize"`
	CommentsSize                   int64       `json:"commentsSize"`
	WhiteSpacesSize                int64       `json:"whiteSpacesSize"`
	DOMelementsCount               int64       `json:"DOMelementsCount"`
	DOMelementMaxDepth             int64       `json:"DOMelementMaxDepth"`
	NodesWithInlineCSS             int64       `json:"nodesWithInlineCSS"`
	IframesCount                   int64       `json:"iframesCount"`
	ImagesScaledDown               int64       `json:"imagesScaledDown"`
	ImagesWithoutDimensions        int64       `json:"imagesWithoutDimensions"`
	DOMidDuplicated                int64       `json:"DOMidDuplicated"`
	HiddenContentSize              int64       `json:"hiddenContentSize"`
	HiddenImages                   int64       `json:"hiddenImages"`
	DOMmutationsInserts            int64       `json:"DOMmutationsInserts"`
	DOMmutationsRemoves            int64       `json:"DOMmutationsRemoves"`
	DOMmutationsAttributes         int64       `json:"DOMmutationsAttributes"`
	DOMqueries                     int64       `json:"DOMqueries"`
	DOMqueriesWithoutResults       int64       `json:"DOMqueriesWithoutResults"`
	DOMqueriesByID                 int64       `json:"DOMqueriesById"`
	DOMqueriesByClassName          int64       `json:"DOMqueriesByClassName"`
	DOMqueriesByTagName            int64       `json:"DOMqueriesByTagName"`
	DOMqueriesByQuerySelectorAll   int64       `json:"DOMqueriesByQuerySelectorAll"`
	DOMinserts                     int64       `json:"DOMinserts"`
	DOMqueriesDuplicated           int64       `json:"DOMqueriesDuplicated"`
	DOMqueriesAvoidable            int64       `json:"DOMqueriesAvoidable"`
	Domains                        int64       `json:"domains"`
	MaxRequestsPerDomain           int64       `json:"maxRequestsPerDomain"`
	MedianRequestsPerDomain        float64     `json:"medianRequestsPerDomain"`
	EventsBound                    int64       `json:"eventsBound"`
	EventsDispatched               int64       `json:"eventsDispatched"`
	EventsScrollBound              int64       `json:"eventsScrollBound"`
	GlobalVariables                int64       `json:"globalVariables"`
	GlobalVariablesFalsy           int64       `json:"globalVariablesFalsy"`
	HeadersCount                   int64       `json:"headersCount"`
	HeadersSentCount               int64       `json:"headersSentCount"`
	HeadersRecvCount               int64       `json:"headersRecvCount"`
	HeadersSize                    int64       `json:"headersSize"`
	HeadersSentSize                int64       `json:"headersSentSize"`
	HeadersRecvSize                int64       `json:"headersRecvSize"`
	HeadersBiggerThanContent       int64       `json:"headersBiggerThanContent"`
	JQueryVersion                  string      `json:"jQueryVersion"`
	JQueryVersionsLoaded           int64       `json:"jQueryVersionsLoaded"`
	JQueryOnDOMReadyFunctions      int64       `json:"jQueryOnDOMReadyFunctions"`
	JQueryWindowOnLoadFunctions    int64       `json:"jQueryWindowOnLoadFunctions"`
	JQuerySizzleCalls              int64       `json:"jQuerySizzleCalls"`
	JQueryEventTriggers            int64       `json:"jQueryEventTriggers"`
	JQueryDOMReads                 int64       `json:"jQueryDOMReads"`
	JQueryDOMWrites                int64       `json:"jQueryDOMWrites"`
	JQueryDOMWriteReadSwitches     int64       `json:"jQueryDOMWriteReadSwitches"`
	DocumentWriteCalls             int64       `json:"documentWriteCalls"`
	EvalCalls                      int64       `json:"evalCalls"`
	JSErrors                       int64       `json:"jsErrors"`
	ClosedConnections              int64       `json:"closedConnections"`
	LazyLoadableImagesBelowTheFold int64       `json:"lazyLoadableImagesBelowTheFold"`
	LocalStorageEntries            int64       `json:"localStorageEntries"`
	MainDomainHTTPProtocol         string      `json:"mainDomainHttpProtocol"`
	OldHTTPProtocol                int64       `json:"oldHttpProtocol"`
	MainDomainTLSProtocol          interface{} `json:"mainDomainTlsProtocol"`
	OldTLSProtocol                 int64       `json:"oldTlsProtocol"`
	Redirects                      int64       `json:"redirects"`
	RedirectsTime                  int64       `json:"redirectsTime"`
	RequestsToFirstPaint           int64       `json:"requestsToFirstPaint"`
	DomainsToFirstPaint            int64       `json:"domainsToFirstPaint"`
	RequestsToDOMContentLoaded     int64       `json:"requestsToDomContentLoaded"`
	DomainsToDOMContentLoaded      int64       `json:"domainsToDomContentLoaded"`
	RequestsToDOMComplete          int64       `json:"requestsToDomComplete"`
	DomainsToDOMComplete           int64       `json:"domainsToDomComplete"`
	AssetsNotGzipped               int64       `json:"assetsNotGzipped"`
	AssetsWithQueryString          int64       `json:"assetsWithQueryString"`
	AssetsWithCookies              int64       `json:"assetsWithCookies"`
	SmallImages                    int64       `json:"smallImages"`
	SmallCSSFiles                  int64       `json:"smallCssFiles"`
	SmallJSFiles                   int64       `json:"smallJsFiles"`
	MultipleRequests               int64       `json:"multipleRequests"`
	TimeToFirstCSS                 *float64    `json:"timeToFirstCss"`
	TimeToFirstJS                  *float64    `json:"timeToFirstJs"`
	TimeToFirstImage               *float64    `json:"timeToFirstImage"`
	DOMInteractive                 int64       `json:"domInteractive"`
	DOMContentLoaded               int64       `json:"domContentLoaded"`
	DOMContentLoadedEnd            int64       `json:"domContentLoadedEnd"`
	DOMComplete                    int64       `json:"domComplete"`
	PerformanceTimingConnect       int64       `json:"performanceTimingConnect"`
	PerformanceTimingDNS           int64       `json:"performanceTimingDNS"`
	PerformanceTimingPageLoad      int64       `json:"performanceTimingPageLoad"`
	PerformanceTimingTTFB          int64       `json:"performanceTimingTTFB"`
	TimeBackend                    int64       `json:"timeBackend"`
	TimeFrontend                   int64       `json:"timeFrontend"`
	StatusCodesTrail               string      `json:"statusCodesTrail"`
	LayoutCount                    int64       `json:"layoutCount"`
	LayoutDuration                 int64       `json:"layoutDuration"`
	RecalcStyleCount               int64       `json:"recalcStyleCount"`
	RecalcStyleDuration            int64       `json:"recalcStyleDuration"`
	ScriptDuration                 int64       `json:"scriptDuration"`
	TaskDuration                   int64       `json:"taskDuration"`
	SmallestResponse               int64       `json:"smallestResponse"`
	BiggestResponse                int64       `json:"biggestResponse"`
	FastestResponse                float64     `json:"fastestResponse"`
	SlowestResponse                float64     `json:"slowestResponse"`
	SmallestLatency                float64     `json:"smallestLatency"`
	BiggestLatency                 float64     `json:"biggestLatency"`
	MedianResponse                 float64     `json:"medianResponse"`
	MedianLatency                  float64     `json:"medianLatency"`
}

type phantomasOutput struct {
	Metrics Metrics
}

func preparePhantomas(node *kubo.Node) error {
	_, err := node.Run(cluster.StartProcRequest{
		Command: "docker",
		Args: []string{
			"pull",
			"macbre/phantomas:latest",
		},
	})
	if err != nil {
		return fmt.Errorf("pulling docker image: %w", err)
	}
	return err
}

func runPhantomas(ctx context.Context, node *kubo.Node, url string) (*Metrics, error) {
	ctx, cancelCurl := context.WithTimeout(ctx, 5*time.Minute)
	defer cancelCurl()

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	_, err := node.Run(cluster.StartProcRequest{
		Command: "docker",
		Args: []string{
			"run",
			"--network=host",
			"--privileged",
			"macbre/phantomas:latest",
			"/opt/phantomas/bin/phantomas.js",
			"--timeout=60",
			fmt.Sprintf("--url=%s", url),
		},
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: stdout: %s, stderr: %s", err, stdout, stderr)
	}

	out := &phantomasOutput{}
	err = json.Unmarshal(stdout.Bytes(), out)
	if err != nil {
		return nil, err
	}
	return &out.Metrics, nil
}
