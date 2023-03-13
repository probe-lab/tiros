package main

import (
	"encoding/json"

	"github.com/volatiletech/null/v8"
)

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
