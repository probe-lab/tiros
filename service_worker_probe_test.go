package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_parseServerTiming(t *testing.T) {
	tests := []struct {
		raw  string
		want map[string]serverTiming
	}{
		{
			raw: "ipfs.resolve;dur=0.0;desc=\"\",exporter-dir;dur=0.0;desc=\"\"",
			want: map[string]serverTiming{
				"ipfs.resolve": {name: "ipfs.resolve", value: 0, desc: "", extra: map[string]string{}},
				"exporter-dir": {name: "exporter-dir", value: 0, desc: "", extra: map[string]string{}},
			},
		},
		{
			raw: "custom-metric;dur=123.45;desc=\"My custom metric\"",
			want: map[string]serverTiming{
				"custom-metric": {name: "custom-metric", value: 123450 * time.Microsecond, desc: "My custom metric", extra: map[string]string{}},
			},
		},
		{
			raw: "cpu;dur=2.4",
			want: map[string]serverTiming{
				"cpu": {name: "cpu", value: 2400 * time.Microsecond, extra: map[string]string{}},
			},
		},
		{
			raw: "cache;desc=\"Cache Read\";dur=23.2",
			want: map[string]serverTiming{
				"cache": {name: "cache", value: 23200 * time.Microsecond, desc: "Cache Read", extra: map[string]string{}},
			},
		},
		{
			raw: "db;dur=53, app;dur=47.2",
			want: map[string]serverTiming{
				"db":  {name: "db", value: 53000 * time.Microsecond, extra: map[string]string{}},
				"app": {name: "app", value: 47200 * time.Microsecond, extra: map[string]string{}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := parseServerTiming(tt.raw)
			assert.Equalf(t, tt.want, got, "parseServerTiming(%v)", tt.raw)
		})
	}
}
