package tiros

import (
	"encoding/json"
	"fmt"
)

const jsPerformanceEntries = `
	async () => {
		const perfEntries = window.performance.getEntries();
		
		function aggregatePerformanceEntries() {
			return new Promise(resolve => {
				const observer = new PerformanceObserver((list) => {
					const lcpEntries = list.getEntries();
					const last = lcpEntries.splice(-1);
					const allEntries = [...perfEntries, ...last]
					resolve(JSON.stringify(allEntries));
				});
				observer.observe({ type: "largest-contentful-paint", buffered: true });
			});
		}
		
		return await aggregatePerformanceEntries();
	}
`

type PerformanceEntry struct {
	Name      string  `json:"name"`
	EntryType string  `json:"entryType"`
	StartTime float64 `json:"startTime"`
	Duration  float64 `json:"duration"`
	Raw       json.RawMessage
}

func unmarshalPerformanceEntries(data []byte) ([]*PerformanceEntry, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, err
	}

	entries := make([]*PerformanceEntry, len(raws))
	for i, raw := range raws {
		pe := PerformanceEntry{}
		if err := json.Unmarshal(raw, &pe); err != nil {
			return nil, err
		}

		pe.Raw = raw

		entries[i] = &pe
	}

	return entries, nil
}

func (pe PerformanceEntry) NavigationEntry() (*PerformanceNavigationEntry, error) {
	if pe.EntryType != "navigation" {
		return nil, fmt.Errorf("performance entry must be %q", "navigation")
	}

	pne := &PerformanceNavigationEntry{}
	if err := json.Unmarshal(pe.Raw, &pne); err != nil {
		return nil, err
	}

	return pne, nil
}

func (pe PerformanceEntry) LargestContentfulPaintEntry() (*PerformanceEntryLargestContentfulPaint, error) {
	if pe.EntryType != "largest-contentful-paint" {
		return nil, fmt.Errorf("performance entry must be %q", "largest-contentful-paint")
	}

	lcpe := &PerformanceEntryLargestContentfulPaint{}
	if err := json.Unmarshal(pe.Raw, &lcpe); err != nil {
		return nil, err
	}

	return lcpe, nil
}

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

type PerformanceEntryLargestContentfulPaint struct {
	Name                   string  `json:"name"`
	EntryType              string  `json:"entryType"`
	StartTime              float64 `json:"startTime"`
	Duration               float64 `json:"duration"`
	Size                   int64   `json:"size"`
	RenderTime             float64 `json:"renderTime"`
	LoadTime               float64 `json:"loadTime"`
	FirstAnimatedFrameTime float64 `json:"firstAnimatedFrameTime"`
	ID                     string  `json:"id"`
	URL                    string  `json:"url"`
}
