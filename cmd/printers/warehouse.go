package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/crypto/cryptoutil"
	"github.com/joonas-fi/bro/pkg/xsfi"
	"github.com/spf13/cobra"
)

var (
	joonasAssets = xsfi.Root.Sub("9") // https://xs.fi/9/<asset ID>
)

type warehouseOptions struct {
	reprint      string
	feed         bool
	barcodeKind  barcodeKind
	barcodes     bool
	texts        bool
	uniqueLabels int
	copies       int
}

func warehouseEntrypoint2() *cobra.Command {
	options := defaultNetumOptions()

	cmd := &cobra.Command{
		Use:   "print-netum [text]",
		Short: "Print rasterized text to a Netum NT-1809DD receipt printer",
		Args:  cobra.ExactArgs(1),
		Run: cli.Runner(func(ctx context.Context, args []string, _ *log.Logger) error {
			return printNetumText(ctx, args[0], options)
		}),
	}

	addNetumFlags(cmd, &options)

	return cmd
}

func warehouseEntrypoint() *cobra.Command {
	reprint := ""
	feed := true
	barcodeKind_ := string(barcodeKindQR)
	barcodes := true
	texts := true
	uniqueLabels := 1
	copies := 1

	cmd := &cobra.Command{
		Use: "warehouse",
		// Short: "Reticulates splines",
		Args: cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(ctx context.Context, _ *log.Logger) error {
			barcodeKind, err := newBarcodeKind(barcodeKind_)
			if err != nil {
				return err
			}

			jobs, err := printWarehouseLabels(ctx, warehouseOptions{
				reprint:      reprint,
				feed:         feed,
				barcodeKind:  barcodeKind,
				barcodes:     barcodes,
				texts:        texts,
				uniqueLabels: uniqueLabels,
				copies:       copies,
			}, defaultIdentifierBuilder, printFileWithFeed)
			if err != nil {
				return err
			}

			for _, job := range jobs {
				fmt.Printf("ID = %s\n", job.LabelHumanReadable)
			}

			return nil
		}),
	}

	cmd.Flags().StringVarP(&reprint, "reprint", "", reprint, "Re-print for an existing ID (don't generate new)")
	cmd.Flags().IntVarP(&uniqueLabels, "unique-labels", "", uniqueLabels, "Amount of unique labels to print")
	cmd.Flags().BoolVarP(&feed, "feed", "", feed, "Feed label out after printing")
	cmd.Flags().StringVarP(&barcodeKind_, "barcode-kind", "", barcodeKind_, "Which kind of barcode to create")
	cmd.Flags().BoolVarP(&barcodes, "barcodes", "", barcodes, "Print barcodes")
	cmd.Flags().BoolVarP(&texts, "texts", "", texts, "Human-readable textual ID")
	cmd.Flags().IntVarP(&copies, "copies", "", copies, "--unique-labels=3, --copies=2 means print 6 labels with 2 copies of each unique label")

	return cmd
}

func printWarehouseLabels(ctx context.Context, options warehouseOptions, buildID identifierBuilder, printImage func(string, bool) error) ([]printJob, error) {
	if options.uniqueLabels < 1 {
		return nil, errors.New("unique labels must be at least 1")
	}
	if options.copies < 1 {
		return nil, errors.New("copies must be at least 1")
	}
	if !options.barcodes && !options.texts {
		return nil, errors.New("barcodes or text must be enabled")
	}

	jobs := makePrintJobs(options.uniqueLabels, options.copies, options.reprint, options.barcodes, options.texts, buildID)
	for idx, job := range jobs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		isLast := idx+1 == len(jobs)
		if err := func() error {
			tempFile, err := os.CreateTemp("", "bro-label-*.png")
			if err != nil {
				return err
			}
			tempFilePath := tempFile.Name()
			defer os.Remove(tempFilePath)

			elements, err := defineContent(job.LabelHumanReadable, job.CodeForBarcode, options.barcodeKind)
			if err == nil {
				err = render(elements, tempFile)
			}
			if closeErr := tempFile.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("render label: %w", err)
			}

			return printImage(tempFilePath, options.feed && isLast)
		}(); err != nil {
			return nil, fmt.Errorf("job %d: %w", idx, err)
		}

		if !isLast {
			// The serial connection is not reliable when jobs arrive back-to-back.
			time.Sleep(3 * time.Second)
		}
	}

	return jobs, nil
}

// describes high-level print job content (label text and its barcode)
type printJob struct {
	LabelHumanReadable string
	CodeForBarcode     string
}

func makePrintJobs(uniqueLabels int, copiesOfEach int, reprint string, barcodes bool, texts bool, buildID identifierBuilder) []printJob {
	jobs := []printJob{}

	for i := 0; i < uniqueLabels; i++ {
		id := func() string {
			if reprint != "" {
				return reprint
			} else {
				return buildID()
			}
		}()

		job := func() printJob {
			codeForBarcode := func() string {
				if !barcodes {
					return ""
				} else {
					return joonasAssets.ID(id).URL()
				}
			}()

			labelHumanReadable := func() string {
				if !texts {
					return ""
				} else {
					if len(id) > 4 {
						return id[0:4] + "\n" + id[4:]
					} else {
						return id
					}
				}
			}()

			return printJob{
				LabelHumanReadable: labelHumanReadable,
				CodeForBarcode:     codeForBarcode,
			}
		}()

		for j := 0; j < copiesOfEach; j++ {
			jobs = append(jobs, job)
		}
	}

	return jobs
}

type identifierBuilder func() string

func defaultIdentifierBuilder() string {
	return cryptoutil.RandBase64UrlWithoutLeadingDash(5)
}
