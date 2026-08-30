package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"slices"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/datamatrix"
	"github.com/boombuler/barcode/qr"
	"github.com/disintegration/imaging"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

type element interface {
	// returns how many pixels the element needs
	Layout() int
	// draw the element's content on the image starting from its offset
	Render(destination draw.Image, offsetX int) draw.Image
}

func defineContent(labelHumanReadable string, codeForBarcode string, barcodeKind_ barcodeKind) ([]element, error) {
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}

	elements := []element{}

	elements = append(elements, &spacerElement{leftMargin})

	txtEl := &textElement{content: strings.Split(labelHumanReadable, "\n"), fontFace: truetype.NewFace(f, &truetype.Options{
		Size:    28,
		DPI:     72,
		Hinting: font.HintingNone, // font.HintingFull
	})}

	hasLabel := labelHumanReadable != ""

	if codeForBarcode != "" {
		elements = append(elements, &barcodeElement{
			content: codeForBarcode,
			encoder: func(content string) (barcode.Barcode, error) {
				switch barcodeKind_ {
				case barcodeKindQR:
					return qr.Encode(content, qr.M, qr.Auto)
				case barcodeKindDatamatrix:
					return datamatrix.Encode(content)
				case barcodeKindCode128:
					return code128.Encode(content)
				default:
					panic("should not happen") // because validated
				}
			},
			// desiredWidthAndHeight: 64,
			desiredWidthAndHeight: 256,
		})
		// FIXME: shouldn't have to care about upcoming element here
		if hasLabel {
			elements = append(elements, &spacerElement{20}) // after barcode
		}
	}

	if hasLabel {
		elements = append(elements, txtEl)
	}

	return elements, nil

}
func render(elements []element, output io.Writer) error {
	width := 0
	for _, el := range elements {
		width += el.Layout()
	}

	var bigger draw.Image = image.NewNRGBA(image.Rect(0, 0, width, drawingHeight))

	width = 0 // reset for 2nd pass

	for _, el := range elements {
		offsetX := width
		bigger = el.Render(bigger, offsetX)

		width += el.Layout()
	}

	return png.Encode(output, bigger)
}

// empty element that just takes up space
type spacerElement struct {
	width int
}

var _ element = (*spacerElement)(nil)

func (m *spacerElement) Layout() int {
	return m.width
}

func (m *spacerElement) Render(destination draw.Image, offsetX int) draw.Image {
	return destination
}

// element where text is drawn
type textElement struct {
	content  []string
	fontFace font.Face
}

var _ element = (*textElement)(nil)

func (t *textElement) Layout() int {
	advances := []int{}
	for _, line := range t.content {
		textAdvance := font.MeasureString(t.fontFace, line)
		advances = append(advances, textAdvance.Round())
	}

	return slices.Max(advances)
}

func (t *textElement) Render(destination draw.Image, offsetX int) draw.Image {
	lineHeight := t.fontFace.Metrics().Height.Round()

	for lineIdx, line := range t.content {
		(&font.Drawer{
			Dst:  destination,
			Src:  image.NewUniform(color.Black),
			Face: t.fontFace,
			Dot: fixed.Point26_6{
				X: fixed.I(offsetX),
				// +ascent because font drawing origin is *bottom* left
				Y: fixed.I(deadZoneTopAndBottom + t.fontFace.Metrics().Ascent.Round() + lineIdx*lineHeight),
			},
		}).DrawString(line)
	}

	return destination
}

type barcodeKind string

const (
	barcodeKindQR         barcodeKind = "qr"
	barcodeKindDatamatrix barcodeKind = "datamatrix"
	barcodeKindCode128    barcodeKind = "code128"
)

func newBarcodeKind(input string) (barcodeKind, error) {
	switch x := barcodeKind(input); x {
	case barcodeKindQR, barcodeKindDatamatrix, barcodeKindCode128:
		return x, nil
	default:
		return "", fmt.Errorf("unrecognized barcodeKind: %s", input)
	}
}

// element that draws a barcode
type barcodeElement struct {
	content               string
	encoder               func(content string) (barcode.Barcode, error)
	desiredWidthAndHeight int
}

var _ element = (*barcodeElement)(nil)

func (b *barcodeElement) Layout() int {
	return b.desiredWidthAndHeight
}

func (b *barcodeElement) Render(destination draw.Image, offsetX int) draw.Image {
	// barcode is first rendered with its native module size being one pixel.
	// if we want it to be bigger, we later scale it (w/o blending to keep it crisp!)
	barcodeImage, err := b.encoder(b.content)
	if err != nil {
		panic(err)
	}

	// scaling internally is implemented so that no undesired blending occurs
	barcodeImageScaled, err := barcode.Scale(barcodeImage, b.desiredWidthAndHeight, b.desiredWidthAndHeight)
	if err != nil {
		panic(err)
	}

	return imaging.Paste(destination, barcodeImageScaled, image.Pt(offsetX, deadZoneTopAndBottom))
}
