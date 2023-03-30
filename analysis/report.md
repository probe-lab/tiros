## Website Monitoring

For a description of our measurement methodology check out [this repository](https://github.com/dennis-tra/tiros).

### Time To First Byte

The time it took to receive the first byte of the first response (that was not a redirect). The large number in each tile is the time in seconds. The number at the very bottom of the graph shows the sample size that went into each subplot/website. Note: the color scales are different in each graph.

![Time To First Byte](./plots/tiros-ttfb.png)

### First Contentful Paint

![First contentful Paint](./plots/tiros-fcp.png)

### Largest Contentful Paint

![Largest contentful Paint](./plots/tiros-lcp.png)

### HTTP vs. Kubo

The number above each bar shows the sample size that went into the calculation.

![HTTP vs. Kubo](./plots/tiros-kubo-vs-http.png)

### Error Rate

The following graph shows the daily error rate in accessing these website.

![Error Rate](./plots/tiros-errors.png)
