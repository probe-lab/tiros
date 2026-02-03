# Tiros

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

Tiros comprises four distinct IPFS performance measurements:

1. Traditional HTTP Gateway Performance
2. Service Worker Gateway Performance
3. Kubo Retrieval and Publication Performance
4. Kubo Website Performance

Each of these measurements is supposed to be run in geographically distributed
regions and requires a different deployment setup. This readme describes each
setup in detail and how the measurements can be run locally.

## Table of Contents

<!-- TOC -->
* [Tiros](#tiros)
  * [Table of Contents](#table-of-contents)
  * [CID Provider Concept](#cid-provider-concept)
  * [Traditional HTTP Gateway Performance](#traditional-http-gateway-performance)
  * [Service Worker Gateway Performance](#service-worker-gateway-performance)
  * [Kubo Retrieval and Publication Performance](#kubo-retrieval-and-publication-performance)
      * [Run](#run)
  * [Kubo Website Performance](#kubo-website-performance)
    * [Measurement Metrics](#measurement-metrics)
    * [Execution](#execution)
  * [Configuration](#configuration)
    * [Global](#global)
    * [Probe](#probe)
    * [Migrations](#migrations)
  * [Testing](#testing)
  * [Maintainers](#maintainers)
  * [Contributing](#contributing)
  * [License](#license)
<!-- TOC -->

## CID Provider Concept

The Traditional HTTP Gateway Performance, Service Worker Gateway Performance,
and Kubo Retrieval and Publication Performance measurements require a set of
CIDs that they request from the network to take measurements. These CIDs are provided by a
CID provider:

```go
type CIDProvider interface {
	SelectCID(ctx context.Context, origin string) (cid.Cid, error)
}
```

Currently, there are five implementations of this interface:

1. `StaticCIDProvider` - a provider that returns a CID from a static list of CIDs
2. `BitswapSnifferClickhouseCIDProvider` - a provider that queries the Clickhouse database of [ProbeLab's BitSwap sniffer](https://github.com/probe-lab/bitswap-sniffer) for the latest discovered CIDs
3. `KuboCIDProvider` (unused) - a provider that queries the IPFS Kubo RPC API for pinned CIDs (useful if we want to probe controlled CIDs as opposed to organically discovered ones)
4. `ControlledCIDProvider` - a provider that returns a CID from a predefined list of CIDs that are hosted on controlled nodes
5. `NoopCIDProvider` - a provider that returns an undefined CID (`cid.Undef`) and never fails

The `StaticCIDProvider` gets its list of CIDs from the `--cids` flag.
This flag expects a comma-separated list of CIDs.

By default, the different performance experiments interleave CIDs from the
`ControlledCIDProvider` with whatever other provider is configured. This can
be disabled with `--controlled.cids=false`.

## Traditional HTTP Gateway Performance

The traditional HTTP Gateway Performance experiment probes the performance of different
HTTP Gateways. The list of gateways to probe can either be provided on the command line
as a comma-separated list of hostnames via the `--gateways` flag or via a
query to a Clickhouse database (default). Then you can provide a list of CIDs to probe
via the `--cids` flag. By default, 20% of requests will randomly be made for controlled CIDs.
To disable this behavior, set `--controlled.cids=false` or `--controlled.share=0`. Conversely,
if you only want to probe controlled CIDs, set `--controlled.share=1`.

To run the traditional HTTP Gateway Performance experiment and store the results in a Clickhouse database, run the following commands:

```shell
docker compose up clickhouse

go run ./cmd/tiros probe gateways --iterations.max 1 --gateways ipfs.io,dweb.link --controlled.share 1 --cids QmUvSqPqYsjeab2JgsNc4PjbAGnCzfn5xid6piJgYYzehH
```

The docker command starts a clickhouse database which will get migrations automatically applied when tiros interacts with the db
for the first time. The user and database are `tiros_local` and the password is `password`. You can connect to the database with the following command:

```shell
clickhouse client --host localhost --password --user tiros_local
```

These are the configuration flags that can be passed to the `probe gateways` command:

```text
NAME:
   tiros probe gateways - Start probing IPFS Gateways retrieval performance

USAGE:
   tiros probe gateways [options]

OPTIONS:
   --interval duration                      How long to wait between each download iteration (default: 10s) [$TIROS_PROBE_GATEWAYS_INTERVAL]
   --iterations.max int                     The number of iterations per concurrent worker to run. 0 means infinite. (default: 0) [$TIROS_PROBE_GATEWAYS_ITERATIONS_MAX]
   --cids string [ --cids string ]          A static list of CIDs to download from the Gateways. [$TIROS_PROBE_GATEWAYS_CIDS]
   --gateways string [ --gateways string ]  A static list of gateways to probe (takes precedence over database) [$TIROS_PROBE_GATEWAYS_GATEWAYS]
   --download.max.mb int                    Maximum download size in MiB before cancelling (default: 10) [$TIROS_PROBE_GATEWAYS_DOWNLOAD_MAX_MB]
   --timeout duration                       Timeout for each gateway request (default: 30s) [$TIROS_PROBE_GATEWAYS_TIMEOUT]
   --refresh.interval duration              How frequently to refresh the gateway list from the database (default: 5m0s) [$TIROS_PROBE_GATEWAYS_REFRESH_INTERVAL]
   --concurrency int                        Number of gateways to probe concurrently (default: 10) [$TIROS_PROBE_GATEWAYS_CONCURRENCY]
   --controlled.cids                        Whether to use the ControlledCIDProvider to select CIDs to probe (default: true) [$TIROS_PROBE_GATEWAYS_CONTROLLED_CIDS]
   --controlled.share float                 What share of requests should be made for controlled CIDs (default: 0.2) [$TIROS_PROBE_GATEWAYS_CONTROLLED_SHARE]
   --help, -h                               show help

GLOBAL OPTIONS:
   --log.level string     Sets an explicit logging level: debug, info, warn, error. (default: "info") [$TIROS_LOG_LEVEL]
   --log.format string    Sets the format to output the log statements in: text, json (default: "text") [$TIROS_LOG_FORMAT]
   --log.source           Compute the source code position of a log statement and add a SourceKey attribute to the output. (default: false) [$TIROS_LOG_SOURCE]
   --metrics.enabled      Whether to expose metrics information (default: false) [$TIROS_METRICS_ENABLED]
   --metrics.host string  Which network interface should the metrics endpoint bind to (default: "localhost") [$TIROS_METRICS_HOST]
   --metrics.port int     On which port should the metrics endpoint listen (default: 6060) [$TIROS_METRICS_PORT]
   --metrics.path string  On which path should the metrics endpoint listen (default: "/metrics") [$TIROS_METRICS_PATH]
   --tracing.enabled      Whether to emit trace data (default: false) [$TIROS_TRACING_ENABLED]
   --aws.region string    On which path should the metrics endpoint listen [$AWS_REGION]
```

## Service Worker Gateway Performance

The Service Worker Gateway Performance experiment probes the performance of the
[Service Worker Gateway](https://github.com/ipfs/service-worker-gateway)
implementation. Tiros orchestrates a headless Chrome instance to load the
Service Worker Gateway and measure the retrieval performance.

To run the Service Worker Gateway Performance experiment and store the results
in a Clickhouse database, run the following commands:

```shell
docker compose up clickhouse chrome

go run ./cmd/tiros probe serviceworker --iterations.max 1 --gateways ipfs.io,dweb.link --controlled.share 1 --cids QmUvSqPqYsjeab2JgsNc4PjbAGnCzfn5xid6piJgYYzehH
```

The docker command starts a clickhouse database which will get migrations automatically applied when tiros interacts with the db
for the first time. The user and database are `tiros_local` and the password is `password`. You can connect to the database with the following command:

```shell
clickhouse client --host localhost --password --user tiros_local
```

These are the configuration flags that can be passed to the `probe serviceworker` command:

```text
NAME:
   tiros probe serviceworker - Start probing IPFS Service Worker Gateway retrieval performance

USAGE:
   tiros probe serviceworker [options]

OPTIONS:
   --interval duration                      How long to wait between each download iteration (default: 10s) [$TIROS_PROBE_GATEWAYS_INTERVAL]
   --iterations.max int                     The number of iterations per concurrent worker to run. 0 means infinite. (default: 0) [$TIROS_PROBE_GATEWAYS_ITERATIONS_MAX]
   --cids string [ --cids string ]          A static list of CIDs to download from the Gateways. [$TIROS_PROBE_GATEWAYS_DOWNLOAD_CIDS]
   --gateways string [ --gateways string ]  A static list of gateways to probe (takes precedence over database) (default: "inbrowser.link") [$TIROS_PROBE_GATEWAYS_GATEWAYS]
   --download.max.mb int                    Maximum download size in MiB before cancelling (default: 10) [$TIROS_PROBE_GATEWAYS_DOWNLOAD_MAX_MB]
   --timeout duration                       Timeout for each gateway request (default: 2m0s) [$TIROS_PROBE_GATEWAYS_TIMEOUT]
   --chrome.cdp.host string                 host at which the Chrome DevTools Protocol is reachable (default: "127.0.0.1") [$TIROS_PROBE_GATEWAYS_CHROME_CDP_HOST]
   --chrome.cdp.port int                    port to reach the Chrome DevTools Protocol port (default: 3000) [$TIROS_PROBE_GATEWAYS_CHROME_CDP_PORT]
   --controlled.cids                        Whether to use the ControlledCIDProvider to select CIDs to probe (default: true) [$TIROS_PROBE_GATEWAYS_CONTROLLED_CIDS]
   --controlled.share float                 What share of requests should be made for controlled CIDs (default: 0.2) [$TIROS_PROBE_GATEWAYS_CONTROLLED_SHARE]
   --help, -h                               show help

GLOBAL OPTIONS:
   --log.level string     Sets an explicit logging level: debug, info, warn, error. (default: "info") [$TIROS_LOG_LEVEL]
   --log.format string    Sets the format to output the log statements in: text, json (default: "text") [$TIROS_LOG_FORMAT]
   --log.source           Compute the source code position of a log statement and add a SourceKey attribute to the output. (default: false) [$TIROS_LOG_SOURCE]
   --metrics.enabled      Whether to expose metrics information (default: false) [$TIROS_METRICS_ENABLED]
   --metrics.host string  Which network interface should the metrics endpoint bind to (default: "localhost") [$TIROS_METRICS_HOST]
   --metrics.port int     On which port should the metrics endpoint listen (default: 6060) [$TIROS_METRICS_PORT]
   --metrics.path string  On which path should the metrics endpoint listen (default: "/metrics") [$TIROS_METRICS_PATH]
   --tracing.enabled      Whether to emit trace data (default: false) [$TIROS_TRACING_ENABLED]
   --aws.region string    On which path should the metrics endpoint listen [$AWS_REGION]
```

## Kubo Retrieval and Publication Performance

The content routing performance measurements are split into the "upload" and "download" parts.

First, you cannot _upload_ anything to IPFS. Instead, what Tiros does is,
it generates random data, calls `ipfs add` and then waits until the provider
record for the root CID was added to the DHT.

The instrumentation itself works by configuring Kubo to emit OpenTelemetry
traces to Tiros itself. So, Tiros interacts with Kubo via the regular RPC API
and all traces then flow back to Tiros. In Tiros, the traces are then captured
and parsed. Irrelevant traces are discarded.

The alternative setup was to pipe the traces to Grafana Cloud (via Alloy for example),
query them from there and analyze them. However, there were three challenges with that:

1. The number of traces was so high that we ran out of our free tier allowance
2. Individual traces were rejected from Grafana Cloud because they were too large
3. Testing the whole pipeline was cumbersome and slow, having everything self-contained simplified e2e testing

However, this setup comes with a few disadvantages as well. First, we're mixing
data collection with data analysis. Raw data will be discarded and we cannot 
extract additional metrics from past collected traces. Second, parsing traces
in Go is not as simple as in Python imo.

#### Run

To run the content routing performance measurement you could do the following:

Terminal 1:
```shell
OTEL_TRACES_EXPORTER=otlp docker compose up kubo 
```

Terminal 2:
```shell
go run ./cmd/tiros probe --json.out out kubo --iterations.max 1 --traces.receiver.host 0.0.0.0 --cids bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi
```

The `--json.out` flag instructs Tiros to write the output to newline-delimited
JSON files in the given directory. In this case `out`. In production, this would
be written to a Clickhouse database.

You can also forward Kubo's traces to, e.g., Jaeger. First start Jaeger:

```text
docker run --rm --name jaeger \
 -e COLLECTOR_OTLP_ENABLED=true \
 -p 16686:16686 \
 -p 55680:4317 \
 cr.jaegertracing.io/jaegertracing/jaeger:2.11.0
```

Then pass the following flags to the above Tiros command:
```
--traces.forward.host 127.0.0.1 --traces.forward.port 55680
```

That way you can inspect the traces that Tiros caught in Jaeger.

### Updating to a new Kubo version

Here are two different CIDs:

- `bafybeigvylgfkdzxw2nxlzlij23ocx73yg77dxtlnb37bg6lo5n34nrrpu` This is a CID hosted on Pinata which isn't indexed in the DHT and instead only accessible via IPNI (unless the Kubo peer is already connected to a Pinata peer or Pinata starts indexing content in the DHT).
- `QmcxHhN5oPuKw8CEmgeSjXeDfnM5o9by4x59xzcSBMnLh5` This is a CID hosted on our controlled Kubo node. This CID is indexed in the DHT and not in IPNI.

Run the following command to gather new test data for IPNI retrievals:

```shell
# start Jaeger
docker run --rm --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 55680:4317 \
  cr.jaegertracing.io/jaegertracing/jaeger:2.11.0
 
# start Tiros
go run ./cmd/tiros probe \
  --json.out out/ipni \
  kubo \
  --download.only \
  --iterations.max 1 \
  --traces.receiver.host 0.0.0.0 \
  --traces.forward.host 127.0.0.1 \
  --traces.forward.port 55680 \
  --cids bafybeigvylgfkdzxw2nxlzlij23ocx73yg77dxtlnb37bg6lo5n34nrrpu \
  --traces.out out/ipni

# start kubo
OTEL_TRACES_EXPORTER=otlp docker compose up kubo
```

As soon as the download operation completes, you should see in the Tiros logs two retrievals.
Next, navigate to the Jaeger UI at http://localhost:16686 where you should see 
two `Kubo: corehttp.cmdsHandler` traces that took slightly longer than others.
Take note of the trace IDs. Then convert them to base64 and search for all corresponding
`out/ipni/trace-*.proto.json` files. When you have figured out in Jaeger that a certain
set of traces belongs to, e.g., an IPNI retrieval, move them into the folder
`./testdata/download_ipni_X` and create an appropriate test case for it in
[`./pkg/kubo/trace_parse_test.go`](./pkg/kubo/trace_parse_test.go).

On top, export the full trace as JSON from Jaeger, so that we can inspect the 
trace again in the future in Jaeger to investigate differences.

Try to find the trace IDs for both retrievals in the `out/ipni/trace-*.proto.json` files by
searching for the CID `bafybeigvylgfkdzxw2nxlzlij23ocx73yg77dxtlnb37bg6lo5n34nrrpu`.

Remove the temporary data by running:

```shell
rm -r out/ipni
```

### Configuration

These are the configuration flags that can be passed to the `probe kubo` command:

```text
NAME:
   tiros probe kubo - Start probing Kubo publication and retrieval performance

USAGE:
   tiros probe kubo [options]

OPTIONS:
   --filesize int                                     File size in MiB to upload to kubo (default: 100) [$TIROS_PROBE_KUBO_UPLOAD_FILE_SIZE_MIB]
   --interval duration                                How long to wait between each upload (default: 10s) [$TIROS_PROBE_KUBO_UPLOAD_INTERVAL]
   --kubo.host string                                 Host at which to reach Kubo (default: "127.0.0.1") [$TIROS_PROBE_KUBO_KUBO_HOST]
   --kubo.api.port int                                port to reach a Kubo-compatible RPC API (default: 5001) [$TIROS_PROBE_KUBO_KUBO_API_PORT]
   --iterations.max int                               The number of iterations to run. 0 means infinite. (default: 0) [$TIROS_PROBE_KUBO_ITERATIONS_MAX]
   --traces.receiver.host string                      The host that the trace receiver is binding to (this is where Kubo should send the traces to) (default: "127.0.0.1") [$TIROS_PROBE_KUBO_TRACES_RECEIVER_HOST]
   --traces.receiver.port int                         The port on which the trace receiver should listen on (this is where Kubo should send the traces to) (default: 4317) [$TIROS_PROBE_KUBO_TRACES_RECEIVER_PORT]
   --traces.out string                                If set, where to write the traces to. [$TIROS_PROBE_KUBO_TRACES_OUT]
   --traces.forward.host string                       The host to forward Kubo's traces to. [$TIROS_PROBE_KUBO_TRACES_FORWARD_HOST]
   --traces.forward.port int                          The port to forward Kubo's traces to. (default: 0) [$TIROS_PROBE_KUBO_TRACES_FORWARD_PORT]
   --cids string [ --static.cids string ]  A static list of CIDs to download from Kubo. [$TIROS_PROBE_KUBO_DOWNLOAD_CIDS]
   --help, -h                                         show help
   --download.only                                    Only download the file from Kubo (default: false) [$TIROS_PROBE_KUBO_DOWNLOAD_ONLY]
   --upload.only                                      Only download the file from Kubo (default: false) [$TIROS_PROBE_KUBO_UPLOAD_ONLY]

GLOBAL OPTIONS:
   --log.level string     Sets an explicit logging level: debug, info, warn, error. (default: "info") [$TIROS_LOG_LEVEL]
   --log.format string    Sets the format to output the log statements in: text, json (default: "text") [$TIROS_LOG_FORMAT]
   --log.source           Compute the source code position of a log statement and add a SourceKey attribute to the output. (default: false) [$TIROS_LOG_SOURCE]
   --metrics.enabled      Whether to expose metrics information (default: false) [$TIROS_METRICS_ENABLED]
   --metrics.host string  Which network interface should the metrics endpoint bind to (default: "localhost") [$TIROS_METRICS_HOST]
   --metrics.port int     On which port should the metrics endpoint listen (default: 6060) [$TIROS_METRICS_PORT]
   --metrics.path string  On which path should the metrics endpoint listen (default: "/metrics") [$TIROS_METRICS_PATH]
   --tracing.enabled      Whether to emit trace data (default: false) [$TIROS_TRACING_ENABLED]
   --aws.region string    On which path should the metrics endpoint listen (default: "/metrics") [$AWS_REGION]
```

## Kubo Website Performance

Each ECS task consists of three containers:

1. `scheduler` (this repository)
2. `chrome` - via [`browserless/chrome`](https://github.com/browserless/chrome)
3. `ipfs` - an IPFS implementation like [ipfs/kubo](https://hub.docker.com/r/ipfs/kubo/) or [ipfs/helia-http-gateway](https://github.com/ipfs/helia-http-gateway)

If run with `kubo` we'll run it with `LIBP2P_RCMGR=0` which disables the [libp2p Network Resource Manager](https://github.com/libp2p/go-libp2p-resource-manager#readme).

The `scheduler` gets configured with a list of websites that will then be probed. A typical website config looks like this `ipfs.io,docs.libp2p.io,ipld.io`.
The scheduler probes each website via the IPFS implementation by requesting `http://localhost:8080/ipns/<website>` and via HTTP by requesting`https://<website>`.
Port `8080` is the default `kubo` HTTP-Gateway port. The `scheduler` uses [`go-rod`](https://github.com/go-rod/rod) to communicate with the `browserless/chrome` instance.
The following excerpt is a gist of what's happening when requesting a website:

```go
browser := rod.New().Context(ctx).ControlURL("ws://localhost:3000")) // default CDP chrome port

browser.Connect()
defer browser.Close()

var metricsStr string
rod.Try(func() {
    browser = browser.Context(c.Context).MustIncognito() // first defense to prevent hitting the cache
    browser.MustSetCookies()                             // second defense to prevent hitting the cache (empty args clears cookies)
    
    page := browser.MustPage() // Get a handle of a new page in our incognito browser
    
    page.MustEvalOnNewDocument(jsOnNewDocument) // third defense to prevent hitting the cache - clears the cache by running `localStorage.clear()`
    
    // disable caching in general
    proto.NetworkSetCacheDisabled{CacheDisabled: true}.Call(page) // fourth defense to prevent hitting the cache


    // finally navigate to url and fail out of rod.Try by panicking
    page.Timeout(websiteRequestTimeout).Navigate(url)
    page.Timeout(websiteRequestTimeout).WaitLoad()
    page.Timeout(websiteRequestTimeout).WaitIdle(time.Minute)

    page.MustEval(wrapInFn(jsTTIPolyfill)) // add TTI polyfill
    page.MustEval(wrapInFn(jsWebVitalsIIFE)) // add web-vitals

    // finally actually measure the stuff
    metricsStr = page.MustEval(jsMeasurement).Str()
    
    page.MustClose()
})
// parse metricsStr
```

`jsOnNewDocument` contains javascript that gets executed on a new page before anything happens. We're subscribing to performance events which is necessary for TTI polyfill and we're clearing the local storage. This is the code ([link to source](https://github.com/probe-lab/tiros/blob/main/js/onNewDocument.js)):

```javascript
// From https://github.com/GoogleChromeLabs/tti-polyfill#usage
!function(){if('PerformanceLongTaskTiming' in window){var g=window.__tti={e:[]};
    g.o=new PerformanceObserver(function(l){g.e=g.e.concat(l.getEntries())});
    g.o.observe({entryTypes:['longtask']})}}();

localStorage.clear();
```

Then, after the website has loaded we are adding a [TTI polyfill](https://github.com/probe-lab/tiros/blob/main/js/tti-polyfill.js) and [web-vitals](https://github.com/probe-lab/tiros/blob/main/js/web-vitals.iife.js) to the page.

We got the tti-polyfill from [GoogleChromeLabs/tti-polyfill](https://github.com/GoogleChromeLabs/tti-polyfill/blob/master/tti-polyfill.js) (archived in favor of the [First Input Delay](https://web.dev/fid/) metric).
We got the web-vitals javascript from [GoogleChrome/web-vitals](https://github.com/GoogleChrome/web-vitals) by building it ourselves with `npm run build` and then copying the `web-vitals.iife.js` (`iife` = immediately invoked function execution)

Then we execute the following javascript on that page ([link to source](https://github.com/probe-lab/tiros/blob/main/js/measurement.js)):

```javascript
async () => {

    const onTTI = async (callback) => {
        const tti = await window.ttiPolyfill.getFirstConsistentlyInteractive({})

        // https://developer.chrome.com/docs/lighthouse/performance/interactive/#how-lighthouse-determines-your-tti-score
        let rating = "good";
        if (tti > 7300) {
            rating = "poor";
        } else if (tti > 3800) {
            rating = "needs-improvement";
        }

        callback({
            name: "TTI",
            value: tti,
            rating: rating,
            delta: tti,
            entries: [],
        });
    };

    const {onCLS, onFCP, onLCP, onTTFB} = window.webVitals;

    const wrapMetric = (metricFn) =>
        new Promise((resolve, reject) => {
            const timeout = setTimeout(() => resolve(null), 10000);
            metricFn(
                (metric) => {
                    clearTimeout(timeout);
                    resolve(metric);
                },
                {reportAllChanges: true}
            );
        });

    const data = await Promise.all([
        wrapMetric(onCLS),
        wrapMetric(onFCP),
        wrapMetric(onLCP),
        wrapMetric(onTTFB),
        wrapMetric(onTTI),
    ]);

    return JSON.stringify(data);
}
```

This function will return a JSON array of the following format:

```json
[
  {
    "name": "CLS",
    "value": 1.3750143983783765e-05,
    "rating": "good",
    ...
  },
  {
    "name": "FCP",
    "value": 872,
    "rating": "good",
    ...
  },
  {
    "name": "LCP",
    "value": 872,
    "rating": "good",
    ...
  },
  {
    "name": "TTFB",
    "value": 717,
    "rating": "good",
    ...
  },
  {
    "name": "TTI",
    "value": 999,
    "rating": "good",
    ...
  }
]
```

If the website request went through the IPFS gateway we're running one round of garbage collection by calling the [`/api/v0/repo/gc` endpoint](https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-repo-gc). With this, we make sure that the next request to that website won't come from the local kubo node cache.

To also measure a "warmed up" kubo node, we also configured a "settle time". This is just the time to wait before the first website requests are made. After the scheduler has looped through all websites we configured another settle time of 10min before all websites are requested again. Each run in between settles also has a "times" counter which is set to `5` right now in our deployment. This means that we request a single website 5 times in between each settle times. The loop looks like this:

```go
for _, settle := range c.IntSlice("settle-times") {
    time.Sleep(time.Duration(settle) * time.Second)
    for i := 0; i < c.Int("times"); i++ {
        for _, mType := range []string{models.MeasurementTypeIPFS, models.MeasurementTypeHTTP} {
            for _, website := range websites {

                pr, _ := t.Probe(c, websiteURL(c, website, mType))
                
                t.Save(c, pr, website, mType, i)

                if mType == models.MeasurementTypeIPFS {
                    t.GarbageCollect(c.Context)
                }
            }
        }
    }
}
```

So in total, each run measures `settle-times * times * len([http, ipfs]) * len(websites)` website requests. In our case it's `2 * 5 * 2 * 14 = 280` requests. This takes around `1h` because some websites time out and the second settle time is configured to be `10m`

These are the configuration flags that can be passed to the `probe website` command:

```text
NAME:
   tiros probe websites - Start probing website performance.

USAGE:
   tiros probe websites [options]

OPTIONS:
   --probes int                             number of times to probe each URL (default: 3) [$TIROS_PROBE_WEBSITES_PROBES]
   --websites string [ --websites string ]  list of websites to probe [$TIROS_PROBE_WEBSITES_WEBSITES]
   --lookup.providers                       Whether to lookup website providers (default: true) [$TIROS_PROBE_WEBSITES_LOOKUP_PROVIDERS]
   --kubo.host string                       Host at which to reach Kubo (default: "127.0.0.1") [$TIROS_PROBE_WEBSITES_KUBO_HOST]
   --kubo.api.port int                      port to reach a Kubo-compatible RPC API (default: 5001) [$TIROS_PROBE_WEBSITES_KUBO_API_PORT]
   --kubo.gateway.port int                  port at which to reach Kubo's HTTP gateway (default: 8080) [$TIROS_PROBE_WEBSITES_KUBO_GATEWAY_PORT]
   --chrome.cdp.host string                 host at which the Chrome DevTools Protocol is reachable (default: "127.0.0.1") [$TIROS_PROBE_WEBSITES_CHROME_CDP_HOST]
   --chrome.cdp.port int                    port to reach the Chrome DevTools Protocol port (default: 3000) [$TIROS_PROBE_WEBSITES_CHROME_CDP_PORT]
   --chrome.kubo.host string                the kubo host from Chrome's perspective. This may be different from Tiros, especially if Chrome and Kubo are run with docker. (default: --kubo.host) [$TIROS_PROBE_WEBSITES_CHROME_KUBO_HOST]
   --help, -h                               show help

GLOBAL OPTIONS:
   --log.level string     Sets an explicit logging level: debug, info, warn, error. (default: "info") [$TIROS_LOG_LEVEL]
   --log.format string    Sets the format to output the log statements in: text, json (default: "text") [$TIROS_LOG_FORMAT]
   --log.source           Compute the source code position of a log statement and add a SourceKey attribute to the output. (default: false) [$TIROS_LOG_SOURCE]
   --metrics.enabled      Whether to expose metrics information (default: false) [$TIROS_METRICS_ENABLED]
   --metrics.host string  Which network interface should the metrics endpoint bind to (default: "localhost") [$TIROS_METRICS_HOST]
   --metrics.port int     On which port should the metrics endpoint listen (default: 6060) [$TIROS_METRICS_PORT]
   --metrics.path string  On which path should the metrics endpoint listen (default: "/metrics") [$TIROS_METRICS_PATH]
   --tracing.enabled      Whether to emit trace data (default: false) [$TIROS_TRACING_ENABLED]
   --aws.region string    On which path should the metrics endpoint listen (default: "/metrics") [$AWS_REGION]
```

### Measurement Metrics

I read up on how to measure website performance and came across this list:

https://developer.mozilla.org/en-US/docs/Learn/Performance/Perceived_performance

To quote the website:

> ## [Performance metrics](https://developer.mozilla.org/en-US/docs/Learn/Performance/Perceived_performance#performance_metrics)
> 
> There is no single metric or test that can be run on a site to evaluate how a user "feels". However, there are a number of metrics that can be "helpful indicators":
> 
> [First paint](https://developer.mozilla.org/en-US/docs/Glossary/First_paint)
> The time to start of first paint operation. Note that this change may not be visible; it can be a simple background color update or something even less noticeable.
> 
> [First Contentful Paint](https://developer.mozilla.org/en-US/docs/Glossary/First_contentful_paint) (FCP)
> The time until first significant rendering (e.g. of text, foreground or background image, canvas or SVG, etc.). Note that this content is not necessarily useful or meaningful.
> 
> [First Meaningful Paint](https://developer.mozilla.org/en-US/docs/Glossary/First_meaningful_paint) (FMP)
> The time at which useful content is rendered to the screen.
> 
> [Largest Contentful Paint](https://wicg.github.io/largest-contentful-paint/) (LCP)
> The render time of the largest content element visible in the viewport.
> 
> [Speed index](https://developer.mozilla.org/en-US/docs/Glossary/Speed_index)
> Measures the average time for pixels on the visible screen to be painted.
> 
> [Time to interactive](https://developer.mozilla.org/en-US/docs/Glossary/Time_to_interactive)
> Time until the UI is available for user interaction (i.e. the last [long task](https://developer.mozilla.org/en-US/docs/Glossary/Long_task) of the load process finishes).

I think the relevant metrics on this list for us are `First Contentful Paint`, `Largest Contentful Paint`, and `Time to interactive`. `First Meaningful Paint` is deprecated (you can see that if you follow the link) and they recommend: "[...] consider using the [LargestContentfulPaint API](https://wicg.github.io/largest-contentful-paint/) instead.".

`First paint` would include changes that "may not be visible", so I'm not particularly fond of this metric.

`Speed index` seems to be very much website-specific. With that, I mean that the network wouldn't play a role in this metric. We would measure the performance of the website itself. I would argue  that this is not something we want.

Besides the above metrics, we should still measure `timeToFirstByte`. According to https://web.dev/ttfb/ the metric would be the time difference between `startTime` and `responseStart`:

![image](https://user-images.githubusercontent.com/11836793/224770610-1a02a082-96e6-4198-8af6-4682d76f2d41.png)

In the above graph you can also see the two timestamps `domContentLoadedEventStart` and `domContentLoadedEventEnd`. So I would think that the `domContentLoaded` metric would just be the difference between the two. However, this seems to only account for the processing time of the HTML ([+ deferred JS scripts](https://developer.mozilla.org/en-US/docs/Web/API/Window/DOMContentLoaded_event)).

We could instead define `domContentLoaded` as the time difference between `startTime` and `domContentLoadedEventEnd`.

### Execution

You need to provide many configuration parameters to `tiros`. See this help page:

## Configuration

### Global

```text
NAME:
   tiros - A new cli application

USAGE:
   tiros [global options] [command [command options]]

COMMANDS:
   probe    Start probing Kubo
   health   Checks the health of the provided endpoint
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help

   Logging Configuration:

   --log.format string  Sets the format to output the log statements in: text, json (default: "text") [$TIROS_LOG_FORMAT]
   --log.level string   Sets an explicit logging level: debug, info, warn, error. (default: "info") [$TIROS_LOG_LEVEL]
   --log.source         Compute the source code position of a log statement and add a SourceKey attribute to the output. (default: false) [$TIROS_LOG_SOURCE]

   Telemetry Configuration:

   --aws.region string    On which path should the metrics endpoint listen (default: "/metrics") [$AWS_REGION]
   --metrics.enabled      Whether to expose metrics information (default: false) [$TIROS_METRICS_ENABLED]
   --metrics.host string  Which network interface should the metrics endpoint bind to (default: "localhost") [$TIROS_METRICS_HOST]
   --metrics.path string  On which path should the metrics endpoint listen (default: "/metrics") [$TIROS_METRICS_PATH]
   --metrics.port int     On which port should the metrics endpoint listen (default: 6060) [$TIROS_METRICS_PORT]
   --tracing.enabled      Whether to emit trace data (default: false) [$TIROS_TRACING_ENABLED]
```

### Probe

```text
NAME:
   tiros probe - Start probing Kubo

USAGE:
   tiros probe [command [command options]]

COMMANDS:
   kubo      Start probing Kubo publication and retrieval performance
   websites  Start probing website performance.

OPTIONS:
   --dry.run           Whether to skip DB interactions (default: false) [$TIROS_PROBE_DRY_RUN]
   --help, -h          show help
   --json.out string   Write measurements to JSON files in the given directory [$TIROS_PROBE_JSON_OUT]
   --timeout duration  The maximum allowed time for this experiment to run (0 no timeout) (default: 0s) [$TIROS_PROBE_TIMEOUT]

   Database Configuration:

   --clickhouse.cluster string                        The cluster name of the Clickhouse service. [$TIROS_PROBE_CLICKHOUSE_CLUSTER]
   --clickhouse.database string                       The ClickHouse database name to connect to (default: "tiros") [$TIROS_PROBE_CLICKHOUSE_DATABASE]
   --clickhouse.host string                           The address where ClickHouse is hosted (default: "127.0.0.1") [$TIROS_PROBE_CLICKHOUSE_HOST]
   --clickhouse.migrations.multiStatement             Whether to use multi-statement mode when applying migrations. (default: false) [$TIROS_PROBE_CLICKHOUSE_MIGRATIONS_MULTI_STATEMENT]
   --clickhouse.migrations.multiStatementMaxSize int  The maximum size of a multi-statement. (default: 10485760) [$TIROS_PROBE_CLICKHOUSE_MIGRATIONS_MULTI_STATEMENT_MAX_SIZE]
   --clickhouse.migrations.replicatedTableEngines     Whether to use replicated table engines. (default: false) [$TIROS_PROBE_CLICKHOUSE_MIGRATIONS_REPLICATED_TABLE_ENGINES]
   --clickhouse.migrationsTable string                The name of the migrations table. (default: "schema_migrations") [$TIROS_PROBE_CLICKHOUSE_MIGRATIONS_TABLE]
   --clickhouse.migrationsTableEngine string          The engine of the migrations table. (default: "TinyLog") [$TIROS_PROBE_CLICKHOUSE_MIGRATIONS_TABLE_ENGINE]
   --clickhouse.password string                       The password for the ClickHouse user (default: "password") [$TIROS_PROBE_CLICKHOUSE_PASSWORD]
   --clickhouse.port int                              Port at which the ClickHouse database is accessible (default: 9000) [$TIROS_PROBE_CLICKHOUSE_PORT]
   --clickhouse.ssl                                   Whether to use SSL to connect to ClickHouse (default: false) [$TIROS_PROBE_CLICKHOUSE_SSL]
   --clickhouse.user string                           The ClickHouse user that has the right privileges (default: "tiros") [$TIROS_PROBE_CLICKHOUSE_USER]
```

### Migrations

To create a new migration run:

```shell
migrate create -dir migrations -ext sql -seq create_website_measurements_table
```

Apply migrations by running:

```shell
migrate -database 'clickhouse://clickhouse_host:9440?username=tiros&password=$PASSWORD&database=tiros&secure=true' -path cmd/tiros/migrations up
```

The migrations will automatically be applied on startup of Tiros. Always define
migrations as if they were applied in a clustered context. Tiros will figure
out how to apply them for a local docker-clickhouse.

## Testing

Tiros has a few end-to-end tests that can be run with:

```shell
just e2e website
just e2e upload
just e2e download
just e2e serviceworker
just e2e gateway
```

## Maintainers

[@probe-lab](https://github.com/probe-lab).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/RichardLitt/standard-readme/issues/new) or submit PRs.

## License

[MIT](LICENSE) © Dennis Trautwein
