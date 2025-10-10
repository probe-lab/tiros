package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func Test_parseIPFSAddTrace(t *testing.T) {
	dat, err := os.ReadFile("./testdata/upload_trace_0.json")
	require.NoError(t, err)

	req := &coltracepb.ExportTraceServiceRequest{}
	require.NoError(t, protojson.Unmarshal(dat, req))

	metrics := parseIPFSAddTrace(req)
	require.NotNil(t, metrics)

	t.Log(metrics.duration.Nanoseconds())

	assert.Equal(t, metrics.duration, time.Duration(5215479333))
	assert.Equal(t, metrics.start, time.Unix(0, 1760112251092659000))
	assert.Equal(t, metrics.end, time.Unix(0, 1760112256308138333))
}

func Test_parseProvideTrace(t *testing.T) {
	dat, err := os.ReadFile("./testdata/upload_trace_1.json")
	require.NoError(t, err)

	req := &coltracepb.ExportTraceServiceRequest{}
	require.NoError(t, protojson.Unmarshal(dat, req))

	metrics := parseProvideTrace(req)
	require.NotNil(t, metrics)

	assert.Equal(t, metrics.duration, time.Duration(10111125417))
	assert.Equal(t, metrics.start, time.Unix(0, 1760112250438997000))
	assert.Equal(t, metrics.end, time.Unix(0, 1760112260550122417))
}
