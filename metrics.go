package main

import (
	"encoding/json"

	"github.com/volatiletech/null/v8"
)

type Metrics struct {
	NavigationPerformance *PerformanceNavigationEntry
	// can grow
}

func (pr *probeResult) NullJSON() (null.JSON, error) {
	m := Metrics{
		NavigationPerformance: pr.navPerf,
	}

	data, err := json.Marshal(m)
	if err != nil {
		return null.NewJSON(nil, false), err
	}

	return null.JSONFrom(data), nil
}

// PerformanceNavigationEntry was generated with quicktype.io
type PerformanceNavigationEntry struct {
	Name                       string  `json:"name"`
	EntryType                  string  `json:"entryType"`
	StartTime                  float64 `json:"startTime"`
	Duration                   float64 `json:"duration"`
	InitiatorType              string  `json:"initiatorType"`
	NextHopProtocol            string  `json:"nextHopProtocol"`
	RenderBlockingStatus       string  `json:"renderBlockingStatus"`
	WorkerStart                float64 `json:"workerStart"`
	RedirectStart              float64 `json:"redirectStart"`
	RedirectEnd                float64 `json:"redirectEnd"`
	FetchStart                 float64 `json:"fetchStart"`
	DomainLookupStart          float64 `json:"domainLookupStart"`
	DomainLookupEnd            float64 `json:"domainLookupEnd"`
	ConnectStart               float64 `json:"connectStart"`
	SecureConnectionStart      float64 `json:"secureConnectionStart"`
	ConnectEnd                 float64 `json:"connectEnd"`
	RequestStart               float64 `json:"requestStart"`
	ResponseStart              float64 `json:"responseStart"`
	ResponseEnd                float64 `json:"responseEnd"`
	TransferSize               int64   `json:"transferSize"`
	EncodedBodySize            int64   `json:"encodedBodySize"`
	DecodedBodySize            int64   `json:"decodedBodySize"`
	ResponseStatus             int64   `json:"responseStatus"`
	UnloadEventStart           float64 `json:"unloadEventStart"`
	UnloadEventEnd             float64 `json:"unloadEventEnd"`
	DOMInteractive             float64 `json:"domInteractive"`
	DOMContentLoadedEventStart float64 `json:"domContentLoadedEventStart"`
	DOMContentLoadedEventEnd   float64 `json:"domContentLoadedEventEnd"`
	DOMComplete                float64 `json:"domComplete"`
	LoadEventStart             float64 `json:"loadEventStart"`
	LoadEventEnd               float64 `json:"loadEventEnd"`
	Type                       string  `json:"type"`
	RedirectCount              int64   `json:"redirectCount"`
	ActivationStart            float64 `json:"activationStart"`
}
