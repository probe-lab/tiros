# Tiros

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

Tiros is an IPFS [Kubo](https://github.com/ipfs/kubo) performance measurement tool.

## Table of Contents


<!-- TOC -->
* [Tiros](#tiros)
  * [Table of Contents](#table-of-contents)
  * [Measurement Methodology](#measurement-methodology)
    * [Content Routing Performance](#content-routing-performance)
      * [Run](#run)
    * [Website Performance](#website-performance)
    * [Measurement Metrics](#measurement-metrics)
    * [Execution](#execution)
      * [Global Configuration](#global-configuration)
      * [Probe Configuration](#probe-configuration)
      * [Website Configuration](#website-configuration)
      * [Content Routing Performance Configuration](#content-routing-performance-configuration)
    * [Migrations](#migrations)
  * [Testing](#testing)
  * [Alternative IPFS Implementation](#alternative-ipfs-implementation)
  * [Maintainers](#maintainers)
  * [Contributing](#contributing)
  * [License](#license)
<!-- TOC -->

## Measurement Methodology

There are two measurement modes to Tiros:

1. Content routing performance measurements
2. Website performance measurements



We, [ProbeLab](https://probelab.io), are running Tiros on AWS ECS task in four different AWS regions. These regions are:

- `eu-central-1`
- `us-east-2`
- `us-west-1`
- `ap-southeast-2`

### Content Routing Performance

The content routing performance measurements are also split into the "upload" and "download" parts.

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
docker compose -f e2e/docker-compose.kubo.yml up 
```

Terminal 2:
```shell
go run . probe --json.out out kubo --iterations.max 1 --traces.receiver.host 0.0.0.0 --download.cids bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi
```

The `--json.out` flag instructs Tiros to write the output to newline-delimited
JSON files in the given directory. In this case `out`. In production, this would
be written to a Clickhouse database.

You can also forward Kubo's traces to, e.g., Jaeger. First start Jaeger:

```text
docker run -d --rm --name jaeger
 -e COLLECTOR_OTLP_ENABLED=true \ 
 -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
 -p 5775:5775/udp \
 -p 6831:6831/udp \
 -p 6832:6832/udp \
 -p 5778:5778 \
 -p 16686:16686 \
 -p 14250:14250 \
 -p 14268:14268 \
 -p 14269:14269 \
 -p 55680:4317 \
 -p 4318:4318 \
 -p 9411:9411 \
 jaegertracing/all-in-one
```

Then pass the following flags to the above Tiros command:
```
---traces.forward.host 127.0.0.1 --traces.forward.port 55680
```

That way you can inspect the traces that Tiros caught in Jaeger.

### Website Performance

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

#### Global Configuration

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

#### Probe Configuration

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

#### Website Configuration

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

#### Content Routing Performance Configuration

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
   --download.cids string [ --download.cids string ]  A static list of CIDs to download from Kubo. [$TIROS_PROBE_KUBO_DOWNLOAD_CIDS]
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

### Migrations

To create a new migration run:

```shell
migrate create -dir migrations -ext sql -seq create_website_measurements_table
```

Apply migrations by running:

```shell
migrate -database 'clickhouse://clickhouse_host:9440?username=tiros&password=$PASSWORD&database=tiros&secure=true' -path cmd/tiros/migrations up
```

## Testing

Tiros has a few end-to-end tests that can be run with:

```shell
just e2e website
just e2e upload
just e2e download
```

## Alternative IPFS Implementation

An alternative IPFS implementation needs to support a couple of things:

1. The [`/api/v0/repo/gc`](https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-repo-gc) endpoint
2. The [`/api/v0/version`](https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-version) endpoint
3. The [`/api/v0/id`](https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-id) endpoint
4. Expose a [rudimentary IPFS Gateway](https://docs.ipfs.tech/reference/http/gateway/) that at least supports resolving IPNS links

## Maintainers

[@dennis-tra](https://github.com/dennis-tra).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/RichardLitt/standard-readme/issues/new) or submit PRs.

## License

[MIT](LICENSE) © Dennis Trautwein
