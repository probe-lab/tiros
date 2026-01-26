package js

import (
	_ "embed"
	"fmt"
)

//go:embed tti-polyfill.js
var TTIPolyfill string

//go:embed web-vitals.iife.js
var WebVitalsIIFE string

//go:embed onNewDocument.js
var OnNewDocument string

//go:embed measurement.js
var Measurement string

//go:embed navPerfEntry.js
var NavPerfEntry string

func WrapInFn(js string) string {
	return fmt.Sprintf("() => { %s };", js)
}

type WebVitals []WebVital

type WebVital struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Rating string  `json:"rating"`
}
