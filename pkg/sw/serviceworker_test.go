package sw

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSwProbe_IsProbeDone(t *testing.T) {
	c, err := cid.Decode("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")
	assert.NoError(t, err)

	// mock probe
	p := NewSwProbe(c, "http://example.com")

	// 1. Initially not done
	assert.False(t, p.isProbeDone())

	// 2. Add a document request (but not final)
	reqID := network.RequestID("req-1")
	loaderID := cdp.LoaderID("loader-1")

	// manually inject request trace
	p.documentRequests[reqID] = &swRequestTrace{
		currentURL: "http://example.com",
		loaderID:   loaderID,
		responses: []*network.Response{
			{
				URL:               "http://example.com",
				Status:            200,
				FromServiceWorker: false, // Not SW
			},
		},
	}
	assert.False(t, p.isProbeDone())

	// 3. Make it a final request candidate (From SW, correct headers)
	// But missing lifecycle event
	p.documentRequests[reqID].responses = append(p.documentRequests[reqID].responses, &network.Response{
		URL:               "http://example.com/ipfs/" + c.String(),
		Status:            200,
		FromServiceWorker: true,
		Headers: network.Headers{
			"x-ipfs-path":                   "/ipfs/" + c.String(),
			"x-ipfs-roots":                  c.String(),
			"access-control-expose-headers": "x-ipfs-path,x-ipfs-roots",
		},
	})
	assert.False(t, p.isProbeDone()) // Missing networkIdle

	// 4. Add networkIdle event
	p.handleLifecycleEvent(&page.EventLifecycleEvent{
		LoaderID: loaderID,
		Name:     "networkIdle",
	})
	assert.True(t, p.isProbeDone())

	// 5. Test attachment/inline content-disposition short-circuit
	reqID2 := network.RequestID("req-2")
	p.documentRequests = make(map[network.RequestID]*swRequestTrace) // reset
	p.documentRequests[reqID2] = &swRequestTrace{
		currentURL: "http://example.com/file",
		loaderID:   loaderID,
		responses: []*network.Response{
			{
				URL:               "http://example.com/file",
				Status:            200,
				FromServiceWorker: true,
				Headers: network.Headers{
					"x-ipfs-path":                   "/ipfs/" + c.String(),
					"x-ipfs-roots":                  c.String(),
					"content-disposition":           "attachment; filename=\"filename.jpg\"",
					"access-control-expose-headers": "x-ipfs-path,x-ipfs-roots,content-disposition",
				},
			},
		},
	}
	// specialized short circuit logic for attachments should return true even without networkIdle
	// (Check code: "if cd, ok := finalResp.Headers["content-disposition"]... if attachment ... return true")
	assert.True(t, p.isProbeDone())
}

func TestSwProbe_BuildProbeResult(t *testing.T) {
	c, err := cid.Decode("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")
	assert.NoError(t, err)

	p := NewSwProbe(c, "http://example.com")

	// Mock a successful flow
	reqID := network.RequestID("req-1")
	loaderID := cdp.LoaderID("loader-1")

	// Request Trace
	p.documentRequests[reqID] = &swRequestTrace{
		currentURL: "http://example.com/ipfs/" + c.String(),
		loaderID:   loaderID,
		responses: []*network.Response{
			{
				URL: "http://example.com/start",
				Timing: &network.ResourceTiming{
					RequestTime: 1000.0,
				},
			},
			{
				URL:               "http://example.com/ipfs/" + c.String(),
				Status:            200,
				FromServiceWorker: true,
				Headers: network.Headers{
					"x-ipfs-path":                   "/ipfs/" + c.String(),
					"x-ipfs-roots":                  c.String(),
					"server-timing":                 "ipfs.resolve;dur=100.0",
					"content-type":                  "text/plain",
					"content-length":                "123",
					"access-control-expose-headers": "x-ipfs-path,x-ipfs-roots,server-timing",
				},
				Timing: &network.ResourceTiming{
					RequestTime:         1001.0,
					ReceiveHeadersStart: 50.0, // relative to RequestTime in ms commonly? No, ResourceTiming RequestTime is baseline in seconds. ReceiveHeadersStart is ms relative to RequestTime.
				},
			},
		},
	}
	p.documentRequestIDs = append(p.documentRequestIDs, reqID)

	// Mock Trustless Gateway request (success)
	p.trustlessGatewayRequests[network.RequestID("gw-1")] = &network.EventResponseReceived{
		Response: &network.Response{
			Status: 200,
			Timing: &network.ResourceTiming{
				ReceiveHeadersStart: 20.0,
			},
		},
	}

	result := p.buildProbeResult()

	// Assertions
	assert.Equal(t, 200, result.FinalStatusCode)
	assert.Equal(t, "/ipfs/"+c.String(), result.IPFSPath)
	assert.Equal(t, "text/plain", result.ContentType)
	assert.Equal(t, int64(123), result.ContentLength)
	assert.True(t, result.ServedFromGateway)

	// Timing checks
	// TimeToFinalRedirect: (1001.0 - 1000.0) * 1e9 ns = 1s
	assert.Equal(t, 1*time.Second, result.TimeToFinalRedirect)

	// FinalTTFB: 50ms
	assert.Equal(t, 50*time.Millisecond, result.FinalTTFB)

	// TotalTTFB: (1001-1000)*1000 + 50 = 1050ms
	assert.Equal(t, 1050*time.Millisecond, result.TotalTTFB)

	// Trustless Gateway TTFB
	assert.Equal(t, 20*time.Millisecond, result.TrustlessGatewayTTFB)

	// Server Timing
	require.Len(t, result.ServerTimings, 1)
	assert.Equal(t, "ipfs.resolve", result.ServerTimings[0].Name)
	assert.Equal(t, 100*time.Millisecond, result.ServerTimings[0].Duration)
}

func TestSwProbe_DelegatedRouterStatus(t *testing.T) {
	c, err := cid.Decode("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")
	assert.NoError(t, err)

	tests := []struct {
		name           string
		responses      []*NetworkEventResponse
		expectedStatus int
	}{
		{
			name: "single success 200",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Body: []byte("{}")},
			},
			expectedStatus: 200,
		},
		{
			name: "single error 502",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 502}}, Body: []byte("{}")},
			},
			expectedStatus: 502,
		},
		{
			name: "single cancel 499 (simulated by Canceled flag)",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Canceled: true, Body: []byte("{}")},
			},
			expectedStatus: 499,
		},
		{
			name: "success 200 + error 502 (success preferred)",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Body: []byte("{}")},
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 502}}, Body: []byte("{}")},
			},
			expectedStatus: 200,
		},
		{
			name: "error 502 + success 200 (success preferred)",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 502}}, Body: []byte("{}")},
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Body: []byte("{}")},
			},
			expectedStatus: 200,
		},
		{
			name: "cancel 499 + error 502 (error preferred over cancel)",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Canceled: true, Body: []byte("{}")},
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 502}}, Body: []byte("{}")},
			},
			expectedStatus: 502,
		},
		{
			name: "error 502 + cancel 499 (error preferred)",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 502}}, Body: []byte("{}")},
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Canceled: true, Body: []byte("{}")},
			},
			expectedStatus: 502,
		},
		{
			name: "cancel 499 + success 200 (success preferred)",
			responses: []*NetworkEventResponse{
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Canceled: true, Body: []byte("{}")},
				{EventResponseReceived: &network.EventResponseReceived{Response: &network.Response{Status: 200}}, Body: []byte("{}")},
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewSwProbe(c, "http://example.com")

			// Inject delegated router responses
			for i, resp := range tt.responses {
				// use fake RequestIDs
				reqID := network.RequestID(string(rune('a' + i))) // "a", "b", ...
				// Provide dummy timing if missing, to avoid nil panics in TTFB calc
				if resp.Response.Timing == nil {
					resp.Response.Timing = &network.ResourceTiming{ReceiveHeadersStart: 10}
				}
				p.delegatedRouterRequests[reqID] = resp
			}

			// Mock minimal document request so BuildProbeResult doesn't exit early
			reqID := network.RequestID("req-doc")
			p.documentRequests[reqID] = &swRequestTrace{
				currentURL: "http://example.com/ipfs/" + c.String(),
				loaderID:   "loader-1",
				responses: []*network.Response{
					{
						URL:    "http://example.com/start",
						Timing: &network.ResourceTiming{RequestTime: 1000},
					},
					{
						URL:               "http://example.com/ipfs/" + c.String(),
						Status:            200,
						FromServiceWorker: true,
						Headers: network.Headers{
							"x-ipfs-path":  "/ipfs/" + c.String(),
							"x-ipfs-roots": c.String(),
						},
						Timing: &network.ResourceTiming{
							RequestTime:         1001,
							ReceiveHeadersStart: 50,
						},
					},
				},
			}
			p.documentRequestIDs = append(p.documentRequestIDs, reqID)

			result := p.buildProbeResult()
			assert.Equal(t, tt.expectedStatus, result.DelegatedRouterStatus)
		})
	}
}
