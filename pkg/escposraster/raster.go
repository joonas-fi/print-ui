// Package escposraster sends 1-bit graphics using the ESC/POS GS v 0 command.
package escposraster

import (
	"fmt"
	"image"
	"image/color"
	"io"
)

const defaultBandHeight = 128

// Options describes the physical raster geometry of a printer.
type Options struct {
	WidthDots  int
	BandHeight int
	Threshold  uint8
}

// Printer streams raster graphics to an ESC/POS-compatible writer.
type Printer struct {
	dst     io.Writer
	options Options
}

func New(dst io.Writer, options Options) (*Printer, error) {
	if options.WidthDots < 1 {
		return nil, fmt.Errorf("print width must be positive")
	}
	if options.BandHeight < 1 {
		options.BandHeight = defaultBandHeight
	}
	if options.Threshold == 0 {
		options.Threshold = 128
	}
	return &Printer{dst: dst, options: options}, nil
}

// Start resets the printer before a graphics-only job.
func (p *Printer) Start() error {
	return writeAll(p.dst, []byte{0x1b, '@'})
}

// PrintImage prints img at the left edge of the configured printhead. The
// remaining dots on each row are white. Images wider than the printhead fail
// instead of being silently cropped.
func (p *Printer) PrintImage(img image.Image) error {
	bounds := img.Bounds()
	if bounds.Dx() > p.options.WidthDots {
		return fmt.Errorf("image width %d exceeds print width %d", bounds.Dx(), p.options.WidthDots)
	}
	for top := 0; top < bounds.Dy(); top += p.options.BandHeight {
		height := min(p.options.BandHeight, bounds.Dy()-top)
		data := p.rasterizeBand(img, bounds.Min.X, bounds.Min.Y+top, bounds.Dx(), height)
		if err := p.writeBand(data, height); err != nil {
			return fmt.Errorf("raster band %d: %w", top/p.options.BandHeight, err)
		}
	}
	return nil
}

// RasterImages returns the exact monochrome bands that PrintImage would send.
// It is useful for previews and diagnostics without writing to a printer.
func (p *Printer) RasterImages(img image.Image) ([]*image.Gray, error) {
	bounds := img.Bounds()
	if bounds.Dx() > p.options.WidthDots {
		return nil, fmt.Errorf("image width %d exceeds print width %d", bounds.Dx(), p.options.WidthDots)
	}
	images := make([]*image.Gray, 0, (bounds.Dy()+p.options.BandHeight-1)/p.options.BandHeight)
	for top := 0; top < bounds.Dy(); top += p.options.BandHeight {
		height := min(p.options.BandHeight, bounds.Dy()-top)
		data := p.rasterizeBand(img, bounds.Min.X, bounds.Min.Y+top, bounds.Dx(), height)
		preview := image.NewGray(image.Rect(0, 0, p.options.WidthDots, height))
		for y := 0; y < height; y++ {
			for x := 0; x < p.options.WidthDots; x++ {
				if data[y*((p.options.WidthDots+7)/8)+x/8]&(0x80>>uint(x%8)) != 0 {
					preview.SetGray(x, y, color.Gray{Y: 0})
				} else {
					preview.SetGray(x, y, color.Gray{Y: 255})
				}
			}
		}
		images = append(images, preview)
	}
	return images, nil
}

// Feed prints white raster rows so paper motion does not rely on a printer's
// text line-height configuration.
func (p *Printer) Feed(rows int) error {
	for rows > 0 {
		height := min(p.options.BandHeight, rows)
		if err := p.writeBand(make([]byte, ((p.options.WidthDots+7)/8)*height), height); err != nil {
			return fmt.Errorf("feed raster band: %w", err)
		}
		rows -= height
	}
	return nil
}

func (p *Printer) rasterizeBand(img image.Image, sourceX, sourceY, sourceWidth, height int) []byte {
	bytesPerRow := (p.options.WidthDots + 7) / 8
	data := make([]byte, bytesPerRow*height)
	for y := 0; y < height; y++ {
		for x := 0; x < sourceWidth; x++ {
			if p.black(img.At(sourceX+x, sourceY+y)) {
				offset := y*bytesPerRow + x/8
				data[offset] |= 0x80 >> (x % 8)
			}
		}
	}
	return data
}

func (p *Printer) writeBand(data []byte, height int) error {
	bytesPerRow := (p.options.WidthDots + 7) / 8
	command := make([]byte, 8+len(data))
	command[0] = 0x1d
	command[1] = 'v'
	command[2] = '0'
	command[3] = 0
	command[4] = byte(bytesPerRow)
	command[5] = byte(bytesPerRow >> 8)
	command[6] = byte(height)
	command[7] = byte(height >> 8)
	copy(command[8:], data)
	return writeAll(p.dst, command)
}

func (p *Printer) black(c interface{ RGBA() (r, g, b, a uint32) }) bool {
	r, g, b, a := c.RGBA()
	// RGBA returns premultiplied components. Composite them onto white first.
	alpha := uint64(a)
	luminance := (uint64(r)*299 + uint64(g)*587 + uint64(b)*114) / 1000
	luminance += 65535 - alpha
	return luminance/257 < uint64(p.options.Threshold)
}

func writeAll(dst io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := dst.Write(data)
		if n > len(data) {
			return fmt.Errorf("writer reported invalid byte count %d", n)
		}
		data = data[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
