package tiros

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestUnmarshalPerformanceEntry(t *testing.T) {
	data := `[{"name":"https://filecoin.io/","entryType":"navigation","startTime":0,"duration":3403,"initiatorType":"navigation","nextHopProtocol":"h2","renderBlockingStatus":"non-blocking","workerStart":0,"redirectStart":0,"redirectEnd":0,"fetchStart":0.20000000001164153,"domainLookupStart":1.4000000000087311,"domainLookupEnd":8.80000000000291,"connectStart":8.80000000000291,"secureConnectionStart":40.5,"connectEnd":79,"requestStart":79.20000000001164,"responseStart":112.40000000000873,"responseEnd":134.5,"transferSize":7465,"encodedBodySize":7165,"decodedBodySize":33603,"responseStatus":0,"serverTiming":[],"unloadEventStart":0,"unloadEventEnd":0,"domInteractive":2920.7000000000116,"domContentLoadedEventStart":2936.9000000000087,"domContentLoadedEventEnd":2936.9000000000087,"domComplete":3402.7000000000116,"loadEventStart":3402.800000000003,"loadEventEnd":3403,"type":"navigate","redirectCount":0,"activationStart":0}]`

	pe, err := unmarshalPerformanceEntries([]byte(data))
	require.NoError(t, err)

	assert.Equal(t, pe[0].Name, "https://filecoin.io/")
	assert.Equal(t, pe[0].EntryType, "navigation")

	pne, err := pe[0].NavigationEntry()
	require.NoError(t, err)
	assert.Equal(t, int(pne.StartTime), 0)
}
