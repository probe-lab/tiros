() => {
    const perfEntries = window.performance.getEntries();
    const navigationPerformanceEntry = perfEntries.find(entry => entry.entryType == "navigation");
    return JSON.stringify(navigationPerformanceEntry);
}