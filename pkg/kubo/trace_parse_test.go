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
		trace := loadTrace(t, fmt.Sprintf("../../testdata/upload_0/trace-%d.proto.json", i))
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
		trace := loadTrace(t, fmt.Sprintf("../../testdata/upload_1/trace-%d.proto.json", i))
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
