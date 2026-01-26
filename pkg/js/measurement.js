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