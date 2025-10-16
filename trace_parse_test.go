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

func Test_parseDownload0Trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("012bfe8c406801383c27e9fd9feefdcd")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("bafkreigmes4fo2xnpixfk4syb5m27iok7rusrh6yziod4y5kunfhb6mf5e"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 2; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/download_0/trace-%d.proto.json", i))
		res.parse(trace)
	}

	provTraceID, err := trace.TraceIDFromHex("8e7418c3caa3097bd02e0b3765c57203")

	assert.True(t, bytes.Equal(res.FindProvTraceID[:], provTraceID[:]))
	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "ipni")
	assert.Equal(t, res.FoundProvidersCount, 3)
	assert.Equal(t, res.ConnectedProvidersCount, 2)
	assert.Equal(t, res.IdleBroadcastStartedAt.UnixNano(), int64(1760508129371731000))
	assert.Equal(t, res.FirstConnectedProviderFoundAt.UnixNano(), int64(1760508129539399000))
	assert.Equal(t, res.FirstProviderConnectedAt.UnixNano(), int64(1760508129664802000))
	assert.Equal(t, res.FirstConnectedProviderPeerID, "QmUA9D3H7HeCYsirB3KmPSvZh3dNXMZas6Lwgr4fv1HTTp")
	assert.Equal(t, res.IPNIStart.UnixNano(), int64(1760508129371835000))
	assert.Equal(t, res.IPNIEnd.UnixNano(), int64(1760508129539239833))
	assert.Equal(t, res.IPNIStatus, 200)
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1760508130285741000))
	assert.True(t, res.cmdHandlerDone)
}

func Test_parseDownload1Trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("c616257b9133a94dbcbc419e2f1dd708")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("QmPrRV2DJHJCneS6Xyjg4y1FkoGidzAbSQxkwjcXi5rpiu"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 3; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/download_1/trace-%d.proto.json", i))
		res.parse(trace)
	}

	provTraceID, err := trace.TraceIDFromHex("25054e5d2a401f3fb89a303f50e88839")

	assert.True(t, bytes.Equal(res.FindProvTraceID[:], provTraceID[:]))
	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "ipni")
	assert.Equal(t, res.FoundProvidersCount, 3)
	assert.Equal(t, res.ConnectedProvidersCount, 3)
	assert.Equal(t, res.IdleBroadcastStartedAt.UnixNano(), int64(1760509237899500000))
	assert.Equal(t, res.FirstConnectedProviderFoundAt.UnixNano(), int64(1760509238114342000))
	assert.Equal(t, res.FirstProviderConnectedAt.UnixNano(), int64(1760509238334902000))
	assert.Equal(t, res.FirstConnectedProviderPeerID, "12D3KooWGkJVfVAQ7uVvmYCdqJRMoKPrPwMgdtL48P5ARXogrteE")
	assert.Equal(t, res.IPNIStart.UnixNano(), int64(1760509237899863000))
	assert.Equal(t, res.IPNIEnd.UnixNano(), int64(1760509238374473083))
	assert.Equal(t, res.IPNIStatus, 200)
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1760509238565359000))
	assert.True(t, res.cmdHandlerDone)
}

func Test_parseDownload2Trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("de73d17d16d1efd3057de66ac224adb3")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("QmUZipvzKLssPTHxUnDwef3a8cPZGL8BwX7urzmNFNtTJ1"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 2; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/download_2/trace-%d.proto.json", i))
		res.parse(trace)
	}

	provTraceID, err := trace.TraceIDFromHex("f03867e56b59fd0772754f8515c4ae22")

	assert.True(t, bytes.Equal(res.FindProvTraceID[:], provTraceID[:]))
	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "dht")
	assert.Equal(t, res.FoundProvidersCount, 1)
	assert.Equal(t, res.ConnectedProvidersCount, 1)
	assert.Equal(t, res.IdleBroadcastStartedAt.UnixNano(), int64(1760509584753297000))
	assert.Equal(t, res.FirstConnectedProviderFoundAt.UnixNano(), int64(1760509584980505000))
	assert.Equal(t, res.FirstProviderConnectedAt.UnixNano(), int64(1760509585055635000))
	assert.Equal(t, res.FirstConnectedProviderPeerID, "12D3KooWJ4kRKuTsCNGF8FzBcmFMVXu4iLvvUiW4EQ2fyU6sVEth")
	assert.Equal(t, res.IPNIStart.UnixNano(), int64(1760509584757540000))
	assert.Equal(t, res.IPNIEnd.UnixNano(), int64(1760509584977517666))
	assert.Equal(t, res.IPNIStatus, 404)
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1760509585080692000))
	assert.True(t, res.cmdHandlerDone)
}

func Test_parseDownload3Trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("c2854351ed71daffd44c6191144faa51")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("QmfAxJ75ePH87jxh6K364P7ce2EFtz3KnU3xzLMmrv3eMN"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 1; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/download_3/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.False(t, res.FindProvTraceID.IsValid())
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
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1760510095230803000))
	assert.True(t, res.cmdHandlerDone)
}
