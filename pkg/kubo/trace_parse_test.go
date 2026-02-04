package kubo

import (
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
	tid, err := trace.TraceIDFromHex("c646a6b29d2a90dae93f180f9ab0b23a")
	require.NoError(t, err)

	c := cid.MustParse("QmZSBqBhnzsbYqm51xRSzYpcVPyyycKQth4Sb3j4z8Ha4a")
	rawCID := cid.NewCidV1(uint64(multicodec.Raw), c.Hash())

	res := UploadResult{
		CID:            c,
		RawCID:         rawCID,
		IPFSAddTraceID: trace.TraceID(tid),
	}

	for i := 0; i < 2; i++ {
		trace := loadTrace(t, fmt.Sprintf("../../testdata/upload_0/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.Equal(t, int64(1770209336864657708), res.IPFSAddStart.UnixNano())
	assert.Equal(t, int64(1770209337914662125), res.IPFSAddEnd.UnixNano())
	assert.Equal(t, int64(1770209337914701417), res.ProvideStart.UnixNano())
	assert.Equal(t, int64(1770209342263130795), res.ProvideEnd.UnixNano())
	assert.False(t, res.ProvideHasErr)
	assert.Nil(t, res.ProvideErr)
	assert.True(t, res.isPopulated())
}

func Test_parse_upload_1_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("6b1c4a51ae99bf5627c510c09825dd2f")
	require.NoError(t, err)

	c := cid.MustParse("QmVxFfdoeeS3makVNch9x2wbjNJ6stDJm56pBmCfmFW69v")
	rawCID := cid.NewCidV1(uint64(multicodec.Raw), c.Hash())

	res := UploadResult{
		CID:            c,
		RawCID:         rawCID,
		IPFSAddTraceID: trace.TraceID(tid),
	}

	for i := 0; i < 3; i++ {
		trace := loadTrace(t, fmt.Sprintf("../../testdata/upload_1/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.Equal(t, int64(1770209353098030716), res.IPFSAddStart.UnixNano())
	assert.Equal(t, int64(1770209354182739675), res.IPFSAddEnd.UnixNano())
	assert.Equal(t, int64(1770209354182781841), res.ProvideStart.UnixNano())
	assert.Equal(t, int64(1770209359312294344), res.ProvideEnd.UnixNano())
	assert.False(t, res.ProvideHasErr)
	assert.Nil(t, res.ProvideErr)
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
