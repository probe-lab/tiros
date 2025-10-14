package main

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func loadTrace(t *testing.T, name string) *ExportTraceServiceRequest {
	dat, err := os.ReadFile(name)
	require.NoError(t, err)
	req := &coltracepb.ExportTraceServiceRequest{}
	require.NoError(t, protojson.Unmarshal(dat, req))
	return &ExportTraceServiceRequest{req}
}

func Test_parseUpload0Trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("ccb58bf33e622255aa65efd4516b5021")
	require.NoError(t, err)

	c := cid.MustParse("bafkreibbo27wv5l7jjquq6aec2ysvxb77riv4yctqn5aoomlr3oskdyrl4")
	rawCID := cid.NewCidV1(uint64(multicodec.Raw), c.Hash())

	res := UploadResult{
		CID:            c,
		RawCID:         rawCID,
		IPFSAddTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 3; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/upload_0/trace-%d.proto.json", i))
		res.parse(trace)
	}

	provTraceID, err := trace.TraceIDFromHex("bd3545bfea892e4919c5f476be5ef93e")
	require.NoError(t, err)

	assert.True(t, bytes.Equal(res.ProvideTraceID[:], provTraceID[:]))
	assert.Equal(t, res.IPFSAddStart.UnixNano(), int64(1760467560212211000))
	assert.Equal(t, res.IPFSAddEnd.UnixNano(), int64(1760467560383873125))
	assert.Equal(t, res.ProvideStart.UnixNano(), int64(1760467560383876000))
	assert.Equal(t, res.ProvideEnd.UnixNano(), int64(1760467571513163084))
	assert.True(t, res.isPopulated())
}

func Test_parseUpload1Trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("aa4784875aff6a43c9f964105de0b184")
	require.NoError(t, err)

	c := cid.MustParse("bafkreif7ndqkrhvnx4l7vsqmdrdis6u2nic4efdc2b53wwyrsobzrbtjwq")
	rawCID := cid.NewCidV1(uint64(multicodec.Raw), c.Hash())

	res := UploadResult{
		CID:            c,
		RawCID:         rawCID,
		IPFSAddTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 5; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/upload_1/trace-%d.proto.json", i))
		res.parse(trace)
	}

	provTraceID, err := trace.TraceIDFromHex("1469a7f3f0d6a68fe98767b31a1cde8a")

	assert.True(t, bytes.Equal(res.ProvideTraceID[:], provTraceID[:]))
	assert.Equal(t, res.IPFSAddStart.UnixNano(), int64(1760468570326433000))
	assert.Equal(t, res.IPFSAddEnd.UnixNano(), int64(1760468570485462250))
	assert.Equal(t, res.ProvideStart.UnixNano(), int64(1760468570485468000))
	assert.Equal(t, res.ProvideEnd.UnixNano(), int64(1760468583575567542))
	assert.True(t, res.isPopulated())
}

//func Test_parseFindProvidersTrace(t *testing.T) {
//	dat, err := os.ReadFile("./testdata/download_trace_1.json")
//	require.NoError(t, err)
//
//	req := &coltracepb.ExportTraceServiceRequest{}
//	require.NoError(t, protojson.Unmarshal(dat, req))
//
//	c := cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")
//	metrics := parseFindProvidersAsyncTrace(req, c)
//	require.NotNil(t, metrics)
//
//	assert.Equal(t, metrics.foundProvidersCount, 7)
//	assert.Equal(t, metrics.connectedProvidersCount, 2)
//	assert.Equal(t, metrics.connectedProviderFoundAt, time.Unix(0, 1760350654906379000))
//	assert.Equal(t, metrics.connectedProviderAt, time.Unix(0, 1760350655029694000))
//}
//
//func Test_delegatedHTTPClientTrace(t *testing.T) {
//	dat, err := os.ReadFile("./testdata/download_trace_0.json")
//	require.NoError(t, err)
//
//	req := &coltracepb.ExportTraceServiceRequest{}
//	require.NoError(t, protojson.Unmarshal(dat, req))
//
//	c := cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")
//	metrics := parseDelegatedHTTPClientTrace(req, c)
//	require.NotNil(t, metrics)
//
//	assert.Equal(t, metrics.statusCode, 200)
//	assert.Equal(t, metrics.start, time.Unix(0, 1760350654813077000))
//	assert.Equal(t, metrics.end, time.Unix(0, 1760350654906312834))
//}
