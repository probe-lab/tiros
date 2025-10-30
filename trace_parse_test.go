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

func Test_parse_download_ipni_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("bce4af7b2a835c3186d36d8743d712f9")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 4; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/download_ipni/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "ipni")
	assert.Equal(t, res.FoundProvidersCount, 66)
	assert.Equal(t, res.ConnectedProvidersCount, 10)
	assert.Equal(t, res.IdleBroadcastStartedAt.UnixNano(), int64(1761817428807744000))
	assert.Equal(t, res.FirstConnectedProviderFoundAt.UnixNano(), int64(1761817428919332000))
	assert.Equal(t, res.FirstProviderConnectedAt.UnixNano(), int64(1761817429051385000))
	assert.Equal(t, res.FirstConnectedProviderPeerID, "QmUA9D3H7HeCYsirB3KmPSvZh3dNXMZas6Lwgr4fv1HTTp")
	assert.Equal(t, res.IPNIStart.UnixNano(), int64(1761817428808716000))
	assert.Equal(t, res.IPNIEnd.UnixNano(), int64(1761817428918260500))
	assert.Equal(t, res.IPNIStatus, 200)
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1761817429773012000))
	assert.True(t, res.cmdHandlerDone)
}

func Test_parse_download_dht_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("a5634cefd8b561f00501fa0e6cecefa3")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("QmUZipvzKLssPTHxUnDwef3a8cPZGL8BwX7urzmNFNtTJ1"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 1; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/download_dht/trace-%d.proto.json", i))
		res.parse(trace)
	}

	assert.True(t, res.isPopulated())
	assert.Equal(t, res.DiscoveryMethod, "dht")
	assert.Equal(t, res.FoundProvidersCount, 1)
	assert.Equal(t, res.ConnectedProvidersCount, 1)
	assert.Equal(t, res.IdleBroadcastStartedAt.UnixNano(), int64(1761818996452685000))
	assert.Equal(t, res.FirstConnectedProviderFoundAt.UnixNano(), int64(1761818996637816000))
	assert.Equal(t, res.FirstProviderConnectedAt.UnixNano(), int64(1761818996701550000))
	assert.Equal(t, res.FirstConnectedProviderPeerID, "12D3KooWJ4kRKuTsCNGF8FzBcmFMVXu4iLvvUiW4EQ2fyU6sVEth")
	assert.Equal(t, res.IPNIStart.UnixNano(), int64(1761818996452973000))
	assert.Equal(t, res.IPNIEnd.UnixNano(), int64(1761818996911612834))
	assert.Equal(t, res.IPNIStatus, 0) // was canceled before response came in
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1761818996730472000))
	assert.True(t, res.cmdHandlerDone)
}

func Test_parse_download_bitswap_trace(t *testing.T) {
	tid, err := trace.TraceIDFromHex("4aa8efb4adc78e4266cbc314e1b7be75")
	require.NoError(t, err)

	res := DownloadResult{
		CID:            cid.MustParse("QmUZipvzKLssPTHxUnDwef3a8cPZGL8BwX7urzmNFNtTJi"),
		IPFSCatTraceID: trace.TraceID(tid),
		spansByTraceID: map[trace.TraceID][]*v1.Span{},
	}

	for i := 0; i < 1; i++ {
		trace := loadTrace(t, fmt.Sprintf("./testdata/download_bitswap/trace-%d.proto.json", i))
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
	assert.Equal(t, res.FirstBlockReceivedAt.UnixNano(), int64(1761819000552233000))
	assert.True(t, res.cmdHandlerDone)
}
