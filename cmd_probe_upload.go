package main

import (
	"context"
	"fmt"
	"math/rand"
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
