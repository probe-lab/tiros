package kubo

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

func Test_parse_upload_0_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("361fc226dcdfb58b7b911f4d5fb1d788")
	require.NoError(t, err)

	c := cid.MustParse("QmetLf6DRxa6rcJe6PeQCSy7g71JtxFZ1TUZu7YyAYTuTQ")
	rawCID := cid.NewCidV1(uint64(multicodec.Raw), c.Hash())

	res := UploadResult{
		CID:             c,
		RawCID:          rawCID,
		IPFSAddTraceID:  trace.TraceID(tid),
		ProvideTraceIDs: make([]trace.TraceID, 0),
		spansByTraceID:  map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 3; i++ {
		trace := loadTrace(t, fmt.Sprintf("../../testdata/upload_0/trace-%d.proto.json", i))
		res.parse(trace)
	}

	provTraceID, err := trace.TraceIDFromHex("26ca5ddc6c7f5e9d7668d0bbd0404be9")
	require.NoError(t, err)

	assert.True(t, bytes.Equal(res.ProvideTraceIDs[0][:], provTraceID[:]))
	assert.Equal(t, res.IPFSAddStart.UnixNano(), int64(1770117225279234512))
	assert.Equal(t, res.IPFSAddEnd.UnixNano(), int64(1770117226287565388))
	assert.Equal(t, res.ProvideStart.UnixNano(), int64(1770117226287644304))
	assert.Equal(t, res.ProvideEnd.UnixNano(), int64(1770117235288959849))
	assert.True(t, res.isPopulated())
}

func Test_parse_upload_1_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("5a8c092976fe983aae6048e8015f3ec7")
	require.NoError(t, err)

	c := cid.MustParse("QmWL7Nva8vovu5GZWCW8LtpcWvdUi3TDyHRpTrUQkfLZhe")
	rawCID := cid.NewCidV1(uint64(multicodec.Raw), c.Hash())

	res := UploadResult{
		CID:             c,
		RawCID:          rawCID,
		IPFSAddTraceID:  trace.TraceID(tid),
		ProvideTraceIDs: make([]trace.TraceID, 0),
		spansByTraceID:  map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 2; i++ {
		trace := loadTrace(t, fmt.Sprintf("../../testdata/upload_1/trace-%d.proto.json", i))
		res.parse(trace)
	}

	provTraceID, err := trace.TraceIDFromHex("81cf7be86352a2cbb82c00b728999f01")
	require.NoError(t, err)

	assert.True(t, bytes.Equal(res.ProvideTraceIDs[1][:], provTraceID[:]))
	assert.Equal(t, res.IPFSAddStart.UnixNano(), int64(1770118410596956588))
	assert.Equal(t, res.IPFSAddEnd.UnixNano(), int64(1770118411596593380))
	assert.Equal(t, res.ProvideStart.UnixNano(), int64(1770118410394565463))
	assert.Equal(t, res.ProvideEnd.UnixNano(), int64(1770118417215926508))
	assert.True(t, res.isPopulated())
}

func Test_parse_download_bitswap_0_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("fec12bc67940d4104117d06bc5351de2")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("bafybeigvylgfkdzxw2nxlzlij23ocx73yg77dxtlnb37bg6lo5n34nrrpu"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 1; i++ {
		trace := loadTrace(t, fmt.Sprintf("../../testdata/download_bitswap_0/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "bitswap")
	assert.Zero(t, res.FoundProvidersCount)
	assert.Zero(t, res.ConnectedProvidersCount)
	assert.True(t, res.IdleBroadcastStartedAt.IsZero())
	assert.True(t, res.FirstConnectedProviderFoundAt.IsZero())
	assert.True(t, res.FirstProviderConnectedAt.IsZero())
	assert.True(t, res.IPNIStart.IsZero())
	assert.True(t, res.IPNIEnd.IsZero())
	assert.Zero(t, res.IPNIStatus)
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1770113728548909463))
	assert.True(t, res.cmdHandlerDone)
}

func Test_parse_download_ipni_0_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("9ee539fca7d18d4279ff9ff26bd3c245")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("bafybeigvylgfkdzxw2nxlzlij23ocx73yg77dxtlnb37bg6lo5n34nrrpu"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 2; i++ {
		trace := loadTrace(t, fmt.Sprintf("../../testdata/download_ipni_0/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "ipni")
	assert.Equal(t, res.FoundProvidersCount, 1)
	assert.Equal(t, res.ConnectedProvidersCount, 1)
	assert.Equal(t, res.IdleBroadcastStartedAt.UnixNano(), int64(1770113721993393585))
	assert.Equal(t, res.FirstConnectedProviderFoundAt.UnixNano(), int64(1770113722100654460))
	assert.Equal(t, res.FirstProviderConnectedAt.UnixNano(), int64(1770113722831927043))
	assert.Equal(t, res.FirstConnectedProviderPeerID, "Qmdv6yNikmUWUWXufLJLRNkv6Y9sY5cmgeX5RVWA4WNMz4")
	assert.Equal(t, res.IPNIStart.UnixNano(), int64(1770113721993679376))
	assert.Equal(t, res.IPNIEnd.UnixNano(), int64(1770113722100527959))
	assert.Equal(t, res.IPNIStatus, 200)
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1770113722958984460))
	assert.True(t, res.cmdHandlerDone)
}

func Test_parse_download_dht_0_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("fcadc115cd766d3d6fec0046976b263b")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("QmcxHhN5oPuKw8CEmgeSjXeDfnM5o9by4x59xzcSBMnLh5"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 2; i++ {
		trace := loadTrace(t, fmt.Sprintf("../../testdata/download_dht_0/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "dht")
	assert.Equal(t, res.FoundProvidersCount, 1)
	assert.Equal(t, res.ConnectedProvidersCount, 1)
	assert.Equal(t, res.IdleBroadcastStartedAt.UnixNano(), int64(1770116364157658835))
	assert.Equal(t, res.FirstConnectedProviderFoundAt.UnixNano(), int64(1770116369784640338))
	assert.Equal(t, res.FirstProviderConnectedAt.UnixNano(), int64(1770116370110620130))
	assert.Equal(t, res.FirstConnectedProviderPeerID, "12D3KooWSa9ut1hY4nTDH7Bq4HUFLj1Q1yBCt9k7jv1HCuzWDTgM")
	assert.Equal(t, res.IPNIStart.UnixNano(), int64(1770116364158600960))
	assert.Equal(t, res.IPNIEnd.UnixNano(), int64(1770116365200769919))
	assert.Equal(t, res.IPNIStatus, 404) // 0 if it was canceled before the response came in
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1770116370219123838))
	assert.True(t, res.cmdHandlerDone)
}
