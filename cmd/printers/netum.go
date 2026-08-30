package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"
	"time"

	"github.com/golang/freetype/truetype"
	"github.com/joonas-fi/bro/pkg/escposraster"
	"github.com/spf13/cobra"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

const (
	netumDefaultWidthDots  = 384
	netumDefaultBandHeight = 128
	// 160 dots is about 20 mm of paper at 203 dpi: over three text line heights
	// beyond the old 48-dot feed, leaving enough paper past the cutter to tear.
	netumDefaultFeedDots   = 160
	netumDefaultSettleTime = 750 * time.Millisecond
	netumFontSize          = 28
	netumLinesPerImage     = 12
)

type netumOptions struct {
	devicePath string
	widthDots  int
	bandHeight int
	feedDots   int
	settleTime time.Duration
}

func defaultNetumOptions() netumOptions {
	return netumOptions{
		devicePath: "/dev/usb/lp0",
		widthDots:  netumDefaultWidthDots,
		bandHeight: netumDefaultBandHeight,
		feedDots:   netumDefaultFeedDots,
		settleTime: netumDefaultSettleTime,
	}
}

func addNetumFlags(cmd *cobra.Command, options *netumOptions) {
	cmd.Flags().StringVarP(&options.devicePath, "device", "d", options.devicePath, "Path to Netum device")
	cmd.Flags().IntVar(&options.widthDots, "width-dots", options.widthDots, "Printable width in dots")
	cmd.Flags().IntVar(&options.bandHeight, "band-height", options.bandHeight, "Raster rows per ESC/POS command")
	cmd.Flags().IntVar(&options.feedDots, "feed-dots", options.feedDots, "Blank raster rows after printing")
	cmd.Flags().DurationVar(&options.settleTime, "settle", options.settleTime, "Wait after a job before another printer command")
}

func printNetumText(ctx context.Context, text string, options netumOptions) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("text must not be empty")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if options.feedDots < 0 {
		return fmt.Errorf("feed dots must not be negative")
	}
	if options.settleTime < 0 {
		return fmt.Errorf("settle time must not be negative")
	}
	device, err := os.OpenFile(options.devicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open printer device: %w", err)
	}
	defer device.Close()

	printer, err := escposraster.New(device, escposraster.Options{
		WidthDots:  options.widthDots,
		BandHeight: options.bandHeight,
	})
	if err != nil {
		return err
	}
	if err := printer.Start(); err != nil {
		return fmt.Errorf("initialize printer: %w", err)
	}
	if err := renderNetumText(text, options.widthDots, func(img image.Image) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return printer.PrintImage(img)
	}); err != nil {
		return fmt.Errorf("render and print text: %w", err)
	}
	if err := printer.Feed(options.feedDots); err != nil {
		return fmt.Errorf("feed paper: %w", err)
	}
	if err := waitForPrinter(ctx, options.settleTime); err != nil {
		return err
	}
	return nil
}

// waitForPrinter keeps a new job from resetting the printer while it is still
// consuming the previous raster data from its internal buffer.
func waitForPrinter(ctx context.Context, duration time.Duration) error {
	if duration == 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func renderNetumText(text string, width int, emit func(image.Image) error) error {
	if width < 1 {
		return fmt.Errorf("print width must be positive")
	}
	face, err := netumFontFace()
	if err != nil {
		return err
	}
	lines := wrapNetumText(text, face, width)
	lineHeight := face.Metrics().Height.Round()
	ascent := face.Metrics().Ascent.Round()
	for len(lines) > 0 {
		count := min(netumLinesPerImage, len(lines))
		chunk := lines[:count]
		height := count * lineHeight
		for idx, line := range chunk {
			bounds, _ := font.BoundString(face, line)
			height = max(height, ascent+idx*lineHeight+bounds.Max.Y.Ceil())
		}
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for idx, line := range chunk {
			(&font.Drawer{
				Dst:  img,
				Src:  image.NewUniform(color.Black),
				Face: face,
				Dot:  fixed.P(0, ascent+idx*lineHeight),
			}).DrawString(line)
		}
		if err := emit(img); err != nil {
			return err
		}
		lines = lines[count:]
	}
	return nil
}

func netumFontFace() (font.Face, error) {
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	return truetype.NewFace(f, &truetype.Options{Size: netumFontSize, DPI: 72, Hinting: font.HintingNone}), nil
}

func wrapNetumText(text string, face font.Face, maxWidth int) []string {
	var wrapped []string
	for _, paragraph := range strings.Split(text, "\n") {
		if strings.TrimSpace(paragraph) == "" {
			wrapped = append(wrapped, "")
			continue
		}
		line := ""
		for _, word := range strings.Fields(paragraph) {
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if font.MeasureString(face, candidate).Round() <= maxWidth {
				line = candidate
				continue
			}
			if line != "" {
				wrapped = append(wrapped, line)
			}
			for font.MeasureString(face, word).Round() > maxWidth {
				prefix, remainder := splitNetumWord(face, word, maxWidth)
				wrapped = append(wrapped, prefix)
				word = remainder
			}
			line = word
		}
		if line != "" {
			wrapped = append(wrapped, line)
		}
	}
	return wrapped
}

func splitNetumWord(face font.Face, word string, maxWidth int) (string, string) {
	prefix := ""
	for _, r := range word {
		candidate := prefix + string(r)
		if prefix != "" && font.MeasureString(face, candidate).Round() > maxWidth {
			return prefix, word[len(prefix):]
		}
		prefix = candidate
	}
	return word, ""
}
