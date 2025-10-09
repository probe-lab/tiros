package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/multiformats/go-multicodec"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var probeUploadConfig = struct {
	Interval    time.Duration
	FileSizeMiB int
}{
	Interval:    time.Minute,
	FileSizeMiB: 100,
}

type firstByteReader struct {
	r             io.Reader
	firstByteTime time.Time
	once          sync.Once
}

func (f *firstByteReader) Read(p []byte) (n int, err error) {
	f.once.Do(func() {
		f.firstByteTime = time.Now()
	})
	return f.r.Read(p)
}

func (f *firstByteReader) FirstByteTime() time.Time {
	return f.firstByteTime
}

var probeUploadCmd = &cli.Command{
	Name:   "upload",
	Action: probeUploadAction,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:        "file-size",
			Usage:       "port to reach the Chrome DevTools Protocol port",
			EnvVars:     []string{"TIROS_PROBE_UPLOAD_FILE_SIZE_MIB"},
			Value:       probeUploadConfig.FileSizeMiB,
			Destination: &probeUploadConfig.FileSizeMiB,
		},
		&cli.DurationFlag{
			Name:        "interval",
			Usage:       "how long to wait between each upload",
			EnvVars:     []string{"TIROS_PROBE_UPLOAD_INTERVAL"},
			Value:       probeUploadConfig.Interval,
			Destination: &probeUploadConfig.Interval,
		},
	},
}

func probeUploadAction(c *cli.Context) error {
	ipfs, err := newKuboClient()
	if err != nil {
		return err
	}

	version, err := kuboVersion(c.Context, ipfs)
	if err != nil {
		return fmt.Errorf("ipfs api offline: %w", err)
	}

	var out IdOutput
	if err := ipfs.Request("id").Exec(c.Context, &out); err != nil {
		return fmt.Errorf("ipfs id: %w", err)
	}

	// Initialize database client
	dbClient, err := newDBClient(c.Context)
	if err != nil {
		return fmt.Errorf("init database client: %w", err)
	}
	tracer := rootConfig.tracerProvider.Tracer("tiros")

	ciid := cid.MustParse("bafkreiazuufr5jxea3gjf5s325owta7ndsufy7neywsy7jqkh5hzrwk7lu")
	p := path.FromCid(ciid)

	pers, err := ipfs.Swarm().Peers(c.Context)
	if err != nil {
		return err
	}
	fmt.Println(pers)

	ctx, cancel := context.WithTimeout(c.Context, 15*time.Second)
	defer cancel()
	rootCtx, rootSpan := tracer.Start(ctx, "download")
	defer rootSpan.End()

	start := time.Now()
	catCtx, catSpan := tracer.Start(rootCtx, "cat")
	defer catSpan.End()
	req := ipfs.Request("cat", p.String())

	reqCtx, reqCancel := context.WithTimeout(catCtx, 15*time.Second)
	defer reqCancel()
	resp, err := req.Send(reqCtx)
	if err != nil {
		return err
	} else if resp.Error != nil {
		return resp.Error
	}
	catSpan.RecordError(err)
	fmt.Println("send return", time.Since(start))

	var buf [1]byte
	_, readSpan := tracer.Start(catCtx, "read.file")
	_, err = resp.Output.Read(buf[:])
	if err != nil {
		return err
	}
	fmt.Println("ttfb", time.Since(start))
	readSpan.AddEvent("ttfb")
	defer readSpan.End()

	data, err := io.ReadAll(resp.Output)
	if err != nil {
		return err
	}
	fmt.Println("done", time.Since(start))
	resp.Output.Close()
	reqCancel()
	data = append(data, buf[:]...)

	return nil

	iteration := 0

	ticker := time.NewTimer(0)
	iterationStart := time.Now()
	var previousPath *path.ImmutablePath
	for {
		iteration += 1

		waitTime := time.Until(iterationStart.Add(probeUploadConfig.Interval)).Truncate(time.Second)
		if waitTime > 0 {
			log.WithField("iteration", iteration).Infof("Waiting %s until the next iteration...", waitTime)
		}

		select {
		case <-c.Context.Done():
			return c.Context.Err()
		case <-ticker.C:
			// continue
		}
		iterationStart = time.Now()

		if previousPath != nil {
			logEntry := log.WithField("cid", previousPath.RootCid().String())
			logEntry.Infoln("Unpinning file from IPFS")
			if err := ipfs.Pin().Rm(c.Context, *previousPath); err != nil {
				logEntry.WithError(err).Warnln("Error unpinning file from IPFS")
			}
			if _, err := ipfs.Request("repo/gc").Send(c.Context); err != nil {
				logEntry.WithError(err).Warnln("Error running ipfs gc")
			}
			previousPath = nil
		}

		// Generate random data
		size := probeUploadConfig.FileSizeMiB * 1024 * 1024
		data := make([]byte, size)
		rand.Read(data)

		ctx, cancel := context.WithTimeout(c.Context, time.Minute)
		ctx, span := tracer.Start(ctx, "upload")
		logEntry := log.WithFields(log.Fields{
			"iteration": iteration,
			"size":      size,
			"traceID":   span.SpanContext().TraceID().String(),
		})
		logEntry.Infoln("Starting next upload iteration")
		imPath, err := ipfs.Unixfs().Add(
			ctx,
			files.NewBytesFile(data),
			options.Unixfs.Pin(true, uuid.NewString()),
			options.Unixfs.FsCache(false),
		)
		span.RecordError(err)
		span.End()
		cancel()

		ticker.Reset(probeUploadConfig.Interval)

		if err != nil {
			logEntry.WithError(err).Warnln("Error adding file to IPFS")
			continue
		}
		previousPath = &imPath

		rawCID := cid.NewCidV1(uint64(multicodec.Raw), imPath.RootCid().Hash())

		logEntry = logEntry.WithFields(log.Fields{
			"cid":    imPath.RootCid().String(),
			"rawCID": rawCID.String(),
		})
		logEntry.Infoln("Uploaded file to Kubo")

		_, err = dbClient.InsertUpload(c, out.ID, version.Version, rootConfig.Region, imPath.RootCid().String(), span.SpanContext().TraceID().String(), size)
		if err != nil {
			return fmt.Errorf("insert upload: %w", err)
		}
	}

	return nil
}
