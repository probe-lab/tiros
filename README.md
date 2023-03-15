# Tiros

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

Tiros is an IPFS website measurement tool. It is intended to run on AWS ECS in multiple regions.

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Run](#run)
- [Development](#development)
- [Maintainers](#maintainers)
- [Contributing](#contributing)
- [License](#license)

## Measurement Methodology

We are running Tiros as a scheduled AWS ECS task in six different AWS regions. These regions are:

- `eu-central-1`

- `ap-south-1`

- `af-southeast-2`

- `sa-east-1`

- `us-west-1`

- `af-south-1`

Each ECS task consists of three containers:

1. `scheduler` (this repository)

2. `chrome` - via [`browserless/chrome`](https://github.com/browserless/chrome)

3. `kubo` - via [ipfs/kubo](https://hub.docker.com/r/ipfs/kubo/)

`kubo` is running with `LIBP2P_RCMGR=0` which disables the [libp2p Network Resource Manager](https://github.com/libp2p/go-libp2p-resource-manager#readme).

The `scheduler` gets configured with a list of websites that will then be probed. A typical website config looks like this `ipfs.io,docs.libp2p.io,ipld.io`. The scheduler probes each website via `kubo` by requesting `http://localhost:8080/ipns/<website>` and `https://<website>`. Port `8080` is the default `kubo` HTTP-Gateway port. The `scheduler` uses [`go-rod`](https://github.com/go-rod/rod) to communicate with the `browserless/chrome` instance. The following excerpt is a gist of what's happening when requesting a website:

```go
browser := rod.New().Context(ctx).ControlURL("ws://localhost:3000")) // default CDP chrome port

browser.Connect()
defer browser.Close()

var perfEntriesStr string
rod.Try(func() {
    browser = browser.MustIncognito() // first defense to prevent hitting the cache
    browser.MustSetCookies()                     // second defense to prevent hitting the cache

    page := browser.MustPage()
    page.MustEvalOnNewDocument(jsOnNewDocument) // third defense to prevent hitting the cache

    proto.NetworkSetCacheDisabled{CacheDisabled: true}.Call(page) // fourth defense to prevent hitting the cache

    page.Timeout(websiteRequestTimeout).Navigate(url)
    page.Timeout(websiteRequestTimeout).WaitLoad() // wait for window.onload
    page.Timeout(websiteRequestTimeout).WaitIdle(time.Minute)
    
    perfEntriesStr = page.MustEval(jsPerformanceEntries).Str()

    page.MustClose()
})
// parse perfEntriesStr
```

`jsOnNewDocument` contains javascript that gets executed on a new page before anything happens. We're adding an event listener that adds error handlers to `link`, `img`, and `script` tags to count HTTP errors. (this is not tracked yet). Then we're also clearing the local storage. This is the code ([link to source](https://github.com/dennis-tra/tiros/blob/871054f38527298aeea6511dc6cdf1683c15f80c/js.go#L11)):

```javascript
window.errorCount = 0

document.addEventListener('readystatechange', () => {
    if (document.readyState != "interactive") {
        return;
    }

    const tags = [
        ...document.querySelectorAll('link'),
        ...document.querySelectorAll('img'),
        ...document.querySelectorAll('script'),
    ]

    tags.map(tag => {
        tag.onerror = () => window.errorCount += 1;
    });
});

localStorage.clear();
```

Then, after the website has loaded we are executing the following javascript on that page ([link to source](https://github.com/dennis-tra/tiros/blob/871054f38527298aeea6511dc6cdf1683c15f80c/js.go#L35)):

```javascript
async () => {
    const perfEntries = window.performance.getEntries();
    
    function aggregatePerformanceEntries() {
        return Promise.race([
            new Promise(resolve => {
                const observer = new PerformanceObserver((list) => {
                    const lcpEntries = list.getEntries();
                    const allEntries = [...perfEntries, ...lcpEntries]
                    resolve(JSON.stringify(allEntries));
                });
                observer.observe({ type: "largest-contentful-paint", buffered: true });
            }),
            new Promise((resolve, reject) => {
                setTimeout(() => resolve(JSON.stringify(perfEntries)), 5000);
            })
        ]);
    }
    
    return await aggregatePerformanceEntries();
}
```

`perfEntries` will contain an array of all [`PerformanceEntry`](https://developer.mozilla.org/en-US/docs/Web/API/PerformanceEntry) objects. This allows us to track the following metrics we're interested in

- `timeToFirstByte` - difference be `responseStart` and `startTime` of the `PerformanceEntry` with `entryType == "navigation"`. This is the [`PerformanceNavigationTiming`](https://developer.mozilla.org/en-US/docs/Web/API/PerformanceNavigationTiming) entry.

- `first-contentful-paint` - `startTime` of the PerformanceEntry of type `first-contentful-paint`

We're also interested in the `largest-contentful-paint` metric. [As the docs say](https://developer.mozilla.org/en-US/docs/Web/API/Performance/getEntries), this isn't part of the `performance.getEntries()` return value. Therefore, we register a performance observer that will resolve a promise with the relevant `PerformanceEntry`'s. To be safe we're racing this promise against atimeout of `5s`. `go-rod` can await these js functions.

If the website request was a request that went through the `kubo` gateway we're running one round of garbage collection by calling the `/api/v0/repo/gc` endpoint. With this we make sure that the next request to that website won't come from the local kubo node cache.



To also measure a "warmed up" kubo node, we also configured a "settle time". This is just the time to wait before the first website requests are made. After the scheduler has looped through all websites we configured another settle time of 10min before all websites are requested again. Each run in between settles also has a "times" counter which is set to `5` right now in our deployment. This means that we request a single website 5 times in between each settle times. The loop looks like this:

```go
for _, settle := range c.IntSlice("settle-times") {
    time.Sleep(time.Duration(settle) * time.Second)
    for i := 0; i < c.Int("times"); i++ {
        for _, mType := range []string{models.MeasurementTypeKUBO, models.MeasurementTypeHTTP} {
            for _, website := range websites {

                pr, _ := t.Probe(c, websiteURL(c, website, mType))
                
                t.Save(c, pr, website, mType, i)

                if mType == models.MeasurementTypeKUBO {
                    t.KuboGC(c.Context)
                }
            }
        }
    }
}
```

So in total, each run measures `settle-times * times * len([http, kubo]) * len(websites)` website requests. In our case it's `2 * 5 * 2 * 14 = 280` requests. This takes around `1h` because some websites time out and the second settle time is configured to be `10m`

## Measurement Metrics

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

## Run

You need to provide many configuration parameters to `tiros`. See this help page:

```text
NAME:
   tiros run

USAGE:
   tiros run [command options] [arguments...]

OPTIONS:
   --websites value [ --websites value ]          Websites to test against. Example: 'ipfs.io' or 'filecoin.io [$TIROS_RUN_WEBSITES]
   --region value                                 In which region does this tiros task run in [$TIROS_RUN_REGION]
   --settle-times value [ --settle-times value ]  a list of times to settle in seconds (default: 10, 1200) [$TIROS_RUN_SETTLE_TIMES]
   --times value                                  number of times to test each URL (default: 3) [$TIROS_RUN_TIMES]
   --db-host value                                On which host address can this clustertest reach the database [$TIROS_RUN_DATABASE_HOST]
   --db-port value                                On which port can this clustertest reach the database (default: 0) [$TIROS_RUN_DATABASE_PORT]
   --db-name value                                The name of the database to use [$TIROS_RUN_DATABASE_NAME]
   --db-password value                            The password for the database to use [$TIROS_RUN_DATABASE_PASSWORD]
   --db-user value                                The user with which to access the database to use [$TIROS_RUN_DATABASE_USER]
   --db-sslmode value                             The sslmode to use when connecting the the database [$TIROS_RUN_DATABASE_SSL_MODE]
   --kubo-api-port value                          port to reach the Kubo API (default: 5001) [$TIROS_RUN_KUBO_API_PORT]
   --kubo-gateway-port value                      port to reach the Kubo Gateway (default: 8080) [$TIROS_RUN_KUBO_GATEWAY_PORT]
   --chrome-cdp-port value                        port to reach the Chrome DevTools Protocol port (default: 3000) [$TIROS_RUN_CHROME_CDP_PORT]
   --cpu value                                    CPU resources for this measurement run (default: 2) [$TIROS_RUN_CPU]
   --memory value                                 Memory resources for this measurement run (default: 4096) [$TIROS_RUN_MEMORY]
   --help, -h                                     show help
```

## Development

To create a new migration run:

```shell
migrate create -ext sql -dir migrations -seq create_measurements_table
```

To create the database models

```shell
make models
```

## Maintainers

[@dennis-tra](https://github.com/dennis-tra).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/RichardLitt/standard-readme/issues/new) or submit PRs.

## License

[MIT](LICENSE) © Dennis Trautwein
