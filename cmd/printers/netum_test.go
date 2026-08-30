package main

import (
	"context"
	"image"
	"testing"
	"time"

	"golang.org/x/image/font"
)

func TestWrapNetumTextWrapsLongWordsAndBlankLines(t *testing.T) {
	face, err := netumFontFace()
	if err != nil {
		t.Fatal(err)
	}
	lines := wrapNetumText("first\n\nverylongword", face, 30)
	if len(lines) < 4 {
		t.Fatalf("lines = %q, want wrapped text with blank line", lines)
	}
	hasBlankLine := false
	for _, line := range lines {
		if line == "" {
			hasBlankLine = true
		}
	}
	if !hasBlankLine {
		t.Fatalf("lines = %q, blank line was not preserved", lines)
	}
	for _, line := range lines {
		if line != "" && font.MeasureString(face, line).Round() > 30 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}

func TestWaitForPrinterHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForPrinter(ctx, time.Second); err != context.Canceled {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderNetumTextProducesBoundedImages(t *testing.T) {
	text := "line\n"
	for i := 0; i < 30; i++ {
		text += "line\n"
	}
	images := 0
	if err := renderNetumText(text, netumDefaultWidthDots, func(img image.Image) error {
		images++
		if img.Bounds().Dy() > netumLinesPerImage*40 {
			t.Fatalf("image height = %d", img.Bounds().Dy())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if images < 3 {
		t.Fatalf("images = %d, want multiple", images)
	}
}

func TestRenderNetumTextKeepsTallGlyphBottom(t *testing.T) {
	face, err := netumFontFace()
	if err != nil {
		t.Fatal(err)
	}
	bounds, _ := font.BoundString(face, "[ ] test")
	wantHeight := face.Metrics().Ascent.Round() + bounds.Max.Y.Ceil()
	var rendered image.Image
	if err := renderNetumText("[ ] test", netumDefaultWidthDots, func(img image.Image) error {
		rendered = img
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if rendered.Bounds().Dy() < wantHeight {
		t.Fatalf("image height = %d, need at least %d for glyph bounds", rendered.Bounds().Dy(), wantHeight)
	}
}
