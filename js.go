package main

import (
	_ "embed"
	"fmt"
)

//go:embed js/tti-polyfill.js
var jsTTIPolyfill string

//go:embed js/check-service-worker-registered.js
var jsCheckServiceWorkerRegistered string

//go:embed js/web-vitals.iife.js
var jsWebVitalsIIFE string

//go:embed js/onNewDocument.js
var jsOnNewDocument string

//go:embed js/measurement.js
var jsMeasurement string

//go:embed js/navPerfEntry.js
var jsNavPerfEntry string

func wrapInFn(js string) string {
	return fmt.Sprintf("() => { %s };", js)
}

type WebVitals []WebVital

type WebVital struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Rating string  `json:"rating"`
}
