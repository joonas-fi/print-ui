package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"

	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/os/osutil"
	"github.com/spf13/cobra"
)

func debugEntrypoint1() *cobra.Command {
	return &cobra.Command{
		Use: "debuggrid",
		// Short: "Reticulates splines",
		Args: cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(ctx context.Context, _ *log.Logger) error {
			// 1st visible pattern at top = 0b10000 = 16
			size := image.Rect(0, 0, 128, 128)
			im := image.NewRGBA(size)

			for y := 0; y < size.Max.Y; y += 2 {
				// 1 => "1"
				yBinary := fmt.Sprintf("%08b", y)
				for binIdx, binDigit := range yBinary {
					if binDigit == '0' { // draw only ones
						continue
					}

					// 4 x Set() => 2x2 square (TODO: use fillRectBlack)
					im.Set(leftMargin+binIdx*2+0, y+0, color.Black)
					im.Set(leftMargin+binIdx*2+1, y+0, color.Black)
					im.Set(leftMargin+binIdx*2+0, y+1, color.Black)
					im.Set(leftMargin+binIdx*2+1, y+1, color.Black)
				}
			}

			return osutil.WriteFileAtomic("y-grid.png", func(writer io.Writer) error {
				return png.Encode(writer, im)
			})
		}),
	}
}

func debugEntrypoint2() *cobra.Command {
	return &cobra.Command{
		Use: "debuggrid2",
		// Short: "Reticulates splines",
		Args: cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(ctx context.Context, _ *log.Logger) error {
			size := image.Rect(0, 0, 128, 128)
			im := image.NewRGBA(size)

			fillRectBlack(im, image.Rect(0, deadZoneTopAndBottom, size.Max.X/2, deadZoneTopAndBottom+1))
			fillRectBlack(im, image.Rect(size.Max.X/2, deadZoneTopAndBottom+1, size.Max.X, deadZoneTopAndBottom+2))

			return osutil.WriteFileAtomic("y-grid2.png", func(writer io.Writer) error {
				return png.Encode(writer, im)
			})
		}),
	}
}
