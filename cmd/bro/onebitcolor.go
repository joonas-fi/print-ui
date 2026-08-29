package main

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/disintegration/imaging"
)

func isBlack(c color.Color) bool {
	r, g, b, a := c.RGBA()
	fullBlack := r == 0 && g == 0 && b == 0
	fullyTransparent := a == 0
	return fullBlack && !fullyTransparent
}

// if alpha has any transparency, light up the pixel (1-bit colors)
func alphaToBlack(c color.Color) bool {
	_, _, _, a := c.RGBA()
	return a > 0
}

func getImageData(imagePath string, isBlack func(color.Color) bool) ([]byte, error) {
	imUnrotated, err := func() (image.Image, error) {
		file, err := os.Open(imagePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		return png.Decode(file)
	}()
	if err != nil {
		return nil, err
	}

	im := imaging.FlipH(imaging.Rotate270(imUnrotated))

	width := im.Bounds().Max.X
	height := im.Bounds().Max.Y

	// /8 because we're using 1-bit colors (8 pixels per byte)

	imageData := make([]byte, width*height/8)
	imageDataByte := 0 // cycles through 0..(width*height/8 - 1)
	imageDataBit := 0  // cycles through 0..7 many times

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if isBlack(im.At(x, y)) {
				imageData[imageDataByte] |= 0b10000000 >> imageDataBit
			}

			imageDataBit++

			if imageDataBit == 8 {
				imageDataByte++
				imageDataBit = 0
			}
		}
	}

	return imageData, nil
}
