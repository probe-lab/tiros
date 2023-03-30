import toml
import pandas as pd
import numpy as np
import seaborn as sns
import matplotlib.pyplot as plt
import matplotlib as mpl
import sqlalchemy as sa
from matplotlib.patches import Patch

sns.set_theme()

DPI = 150


def connection_string(config=toml.load("./db.toml")['psql']) -> str:
    return f"postgresql://{config['user']}:{config['password']}@{config['host']}:{config['port']}/{config['database']}"


def cdf(series: pd.Series) -> pd.DataFrame:
    """ calculates the cumulative distribution function of the given series"""
    return pd.DataFrame.from_dict({
        series.name: np.append(series.sort_values(), series.max()),
        "cdf": np.linspace(0, 1, len(series) + 1)
    })


def get_measurements(conn: sa.engine.Engine, start_date: str, end_date: str) -> pd.DataFrame:
    print("Get measurements...")

    query = f"""
    SELECT
        m.id,
        r.region,
        m.website,
        m.url,
        m.type,
        m.try,
        EXTRACT(EPOCH FROM m.ttfb) ttfb,
        EXTRACT(EPOCH FROM m.fcp) fcp,
        EXTRACT(EPOCH FROM m.lcp) lcp,
        date_trunc('day', m.created_at)::DATE date,
        CASE
            WHEN m.error = 'navigation timed out' THEN
                TRUE
            ELSE
                FALSE
        END has_error,
        m.created_at
    FROM measurements m
        INNER JOIN runs r on m.run_id = r.id
    WHERE m.created_at >= '{start_date}'
      AND m.created_at < '{end_date}'
    ORDER BY m.created_at
    """
    return pd.read_sql_query(query, con=conn)


def get_percentiles(data: pd.DataFrame, percentile: float = 0.5, metric: str = "fcp"):
    print("get_percentiles")

    agg = data[["website", "region", metric]] \
        .groupby(["website", "region"]) \
        .quantile(percentile, numeric_only=True).reset_index()

    row_labels = list(sorted(agg["region"].unique()))
    col_labels = list(sorted(agg["website"].unique()))
    dat = []
    counts = []
    for region in row_labels:
        region_values = []
        region_counts = []
        for website in col_labels:
            region_counts += [data[(data["region"] == region) & (data["website"] == website)].count().iloc[0]]
            series = agg[(agg["region"] == region) & (agg["website"] == website)][metric]
            if len(series) > 0:
                region_values += [series.iloc[0]]
            else:
                region_values += [np.NAN]
        dat += [region_values]
        counts += [region_counts]
    dat = np.array(dat)
    counts = np.array(counts)
    return dat, counts, row_labels, col_labels


def plot_metric(df: pd.DataFrame, title: str, metric: str):
    print("plot_metric")

    fig, axes = plt.subplots(3, 1, figsize=[17, 17])

    pos = None
    for idx, percentile in enumerate([0.5, 0.9, 0.99]):
        cbar_kw = {}

        dat, counts, row_labels, col_labels = get_percentiles(df, percentile, metric)

        ax = fig.axes[idx]

        im = ax.imshow(dat, cmap=sns.color_palette("rocket_r", as_cmap=True))

        # Create colorbar
        cbar = ax.figure.colorbar(im, ax=ax, **cbar_kw)
        cbar.ax.set_ylabel(f"p{int(percentile * 100)} Latency in Seconds", rotation=-90, va="bottom")

        # Show all ticks and label them with the respective list entries.
        if idx == 0:
            ax.set_title(title)
            ax.set_xticks(np.arange(dat.shape[1]), labels=col_labels)
        else:
            ax.set_xticks([], labels=[])

        ax.set_yticks(np.arange(dat.shape[0]), labels=row_labels)

        # Let the horizontal axes labeling appear on top.
        ax.tick_params(top=True, bottom=False, labeltop=True, labelbottom=False)

        # Rotate the tick labels and set their alignment.
        plt.setp(ax.get_xticklabels(), rotation=-30, ha="right", rotation_mode="anchor")

        # Turn spines off and create white grid.
        ax.spines[:].set_visible(False)

        ax.set_xticks(np.arange(dat.shape[1] + 1) - .5, minor=True)
        ax.set_yticks(np.arange(dat.shape[0] + 1) - .5, minor=True)
        ax.grid(False)
        ax.tick_params(which="minor", bottom=False, left=False)

        threshold = im.norm(dat.max()) / 2.
        textcolors = ("#212121", "white")
        kw = dict(ha="center", va="center")
        fmtr = mpl.ticker.StrMethodFormatter("{x:.3f}")
        for i in range(dat.shape[0]):
            for j in range(dat.shape[1]):
                tc = textcolors[int(im.norm(dat[i, j]) > threshold)]
                kw.update(color=tc)
                im.axes.text(j, i, fmtr(dat[i, j]), **kw)

        ax.text(-2, -1.5 if idx == 0 else -0.5, f"p{int(percentile * 100)}", ha="left", va="top", fontweight="bold",
                fontsize="large")

        if idx == 2:
            for j, count in enumerate(np.sum(counts, axis=0)):
                ax.text(j, 6.6, f"Samples\n{count}", ha="center", va="top", fontsize=8)

    fig.tight_layout()

    return fig


def plot_errors(df: pd.DataFrame) -> plt.Figure:
    print("plot_errors")

    fig, ax = plt.subplots(figsize=[15, 7], dpi=150)

    for j, website in enumerate(sorted(df["website"].unique())):
        ls = "solid" if j < 10 else "dashed"
        dat = df[df["website"] == website].copy()
        dat["%"] = 100 * dat["id"] / dat.groupby("date")["id"].transform("sum")
        dat = dat[dat["has_error"]]

        ax.plot(dat["date"], dat["%"], ls=ls, label=website)

    ax.legend(loc='upper center', bbox_to_anchor=(0.5, 1.2), ncols=5)
    ax.set_xlabel("Date")
    ax.set_ylabel("Daily Error Rate in % of All Website Requests / day")

    fig.tight_layout()

    return fig


def plot_kubo_vs_http(df_query: pd.DataFrame) -> plt.Figure:
    print("plot_kubo_vs_http")

    df = df_query.copy() \
        .groupby(["date", "website", "region", "type"]) \
        .median(numeric_only=True) \
        .reset_index()[["date", "website", "region", "type", "ttfb"]]

    metric = "timeToFirstByte"
    region = "eu-central-1"
    websites = list(sorted(df["website"].unique()))

    fig, ax = plt.subplots(3, 1, figsize=[12, 10], dpi=150, sharex=True)

    width = 0.4

    for i, percentile in enumerate([50, 90, 99]):
        ax = fig.axes[i]

        grouped = df.groupby(["website", "region", "type"]).quantile(percentile / 100, numeric_only=True).reset_index()[
            ["website", "region", "type", "ttfb"]]

        values = []

        xticks = []
        labels = []
        for j, website in enumerate(websites):
            dat = grouped.copy()
            dat = dat[dat["website"] == website]
            dat = dat[dat["region"] == region]
            dat_http = dat[dat["type"] == "HTTP"]
            dat_kubo = dat[dat["type"] == "KUBO"]

            samples_kubo = df_query[
                (df_query["website"] == website) & (df_query["type"] == "KUBO") & (
                        df_query["region"] == region)].count()[
                "id"]
            kubo_y = dat_kubo["ttfb"].iloc[0]
            p = ax.bar(j - width, kubo_y, color="b", align="edge", label="KUBO", width=width)
            ax.bar_label(p, labels=[samples_kubo], fontsize=8)

            samples_http = df_query[
                (df_query["website"] == website) & (df_query["type"] == "HTTP") & (
                        df_query["region"] == region)].count()[
                "id"]
            http_y = dat_http.reset_index()["ttfb"].iloc[0]
            p = ax.bar(j, http_y, color="r", align="edge", label="HTTP", width=width)
            ax.bar_label(p, labels=[samples_http], fontsize=8)

            values += [kubo_y, http_y]

            xticks += [j]
            labels += [website]

        for j, website in enumerate(websites):
            kubo_y = values[2 * j]
            http_y = values[2 * j + 1]
            ax.text(j, max(kubo_y, http_y) + 0.07 * np.max(values), f"Ratio\n{kubo_y / http_y:.1f}", ha="center",
                    va="bottom", fontsize=8)

        ax.tick_params(bottom=True)
        ax.set_xticks(xticks, labels)
        for tick in ax.get_xticklabels():
            tick.set_rotation(15)
            tick.set_ha("right")

        if i + 1 == len(websites):
            ax.set_xlabel("Website")

        ax.set_ylabel("Latency in ms")
        ax.set_ylim(0, np.max(values) + 0.18 * np.max(values))

        legend_elements = [
            Patch(facecolor='b', edgecolor='b', label='KUBO'),
            Patch(facecolor='r', edgecolor='r', label='HTTP')
        ]

        ax.legend(title=f"p{percentile}", handles=legend_elements, loc='upper left',
                  title_fontproperties={"weight": "bold"})

    fig.suptitle(
        f"{metric} | {region} | {df_query['date'].min().strftime('%Y-%m-%d')} - {df_query['date'].max().strftime('%Y-%m-%d')}",
        fontsize=16)
    fig.tight_layout()
    return fig


def main():
    conn = sa.create_engine(connection_string())
    date_min = "2023-03-27"
    date_max = "2023-04-03"

    df = get_measurements(conn, date_min, date_max)

    kubo_requests = df[(df["type"] == "KUBO") & (df["has_error"] == False)]

    fig = plot_metric(kubo_requests, "First Contentful Paint", "fcp")
    fig.savefig("./plots/tiros-fcp.png", dpi=DPI)

    fig = plot_metric(kubo_requests, "Largest Contentful Paint", "lcp")
    fig.savefig("./plots/tiros-lcp.png", dpi=DPI)

    fig = plot_metric(kubo_requests, "Time To First Byte", "ttfb")
    fig.savefig("./plots/tiros-ttfb.png", dpi=DPI)

    errors = df[df["type"] == "KUBO"].copy().groupby(["date", "has_error", "website"]) \
        .count() \
        .reset_index()

    fig = plot_errors(errors)
    fig.savefig("./plots/tiros-errors.png", dpi=DPI)

    errors = df[df["type"] == "HTTP"].copy().groupby(["date", "has_error", "website"]) \
        .count() \
        .reset_index()

    fig = plot_errors(errors)
    fig.savefig("./plots/tiros-errors-http.png", dpi=DPI)

    fig = plot_kubo_vs_http(df.copy())
    fig.savefig("./plots/tiros-kubo-vs-http.png", dpi=DPI)


if __name__ == "__main__":
    main()
