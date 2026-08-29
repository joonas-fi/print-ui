package escposraster

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestPrintImagePadsWidthAndSplitsBands(t *testing.T) {
	img := image.NewRGBA(image.Rect(10, 20, 19, 23))
	img.Set(10, 20, color.Black)
	img.Set(18, 22, color.Black)
	var output bytes.Buffer
	p := mustPrinter(t, &output, Options{WidthDots: 9, BandHeight: 2})
	if err := p.PrintImage(img); err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x1d, 'v', '0', 0, 2, 0, 2, 0, 0x80, 0, 0, 0,
		0x1d, 'v', '0', 0, 2, 0, 1, 0, 0, 0x80,
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Errorf("output = %x, want %x", output.Bytes(), want)
	}
}

func TestPrintImageCompositesTransparentPixelsOntoWhite(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 1))
	img.Set(0, 0, color.RGBA{0, 0, 0, 0})
	img.Set(1, 0, color.RGBA{0, 0, 0, 255})
	var output bytes.Buffer
	p := mustPrinter(t, &output, Options{WidthDots: 8})
	if err := p.PrintImage(img); err != nil {
		t.Fatal(err)
	}
	if got := output.Bytes()[8]; got != 0x40 {
		t.Errorf("raster byte = %08b, want 01000000", got)
	}
}

func TestRasterImagesMatchPrintedBits(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 9, 1))
	img.Set(8, 0, color.Black)
	p := mustPrinter(t, &bytes.Buffer{}, Options{WidthDots: 9})
	previews, err := p.RasterImages(img)
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) != 1 || previews[0].GrayAt(8, 0).Y != 0 || previews[0].GrayAt(7, 0).Y != 255 {
		t.Fatalf("unexpected preview pixels")
	}
}

func TestPrintImageRejectsTooWideImage(t *testing.T) {
	p := mustPrinter(t, &bytes.Buffer{}, Options{WidthDots: 8})
	err := p.PrintImage(image.NewRGBA(image.Rect(0, 0, 9, 1)))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteAllHandlesShortWritesAndFailure(t *testing.T) {
	w := &shortWriter{limit: 2}
	if err := writeAll(w, []byte{1, 2, 3, 4, 5}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.data, []byte{1, 2, 3, 4, 5}) {
		t.Errorf("data = %v", w.data)
	}
	if err := writeAll(errorWriter{}, []byte{1}); !errors.Is(err, errWrite) {
		t.Fatalf("error = %v", err)
	}
}

func mustPrinter(t *testing.T, dst *bytes.Buffer, options Options) *Printer {
	t.Helper()
	p, err := New(dst, options)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type shortWriter struct {
	data  []byte
	limit int
}

func (w *shortWriter) Write(data []byte) (int, error) {
	n := min(w.limit, len(data))
	w.data = append(w.data, data[:n]...)
	return n, nil
}

var errWrite = errors.New("write failed")

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errWrite }
