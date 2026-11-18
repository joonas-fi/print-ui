package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"io"
	"net/http"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/datamatrix"
	"github.com/boombuler/barcode/qr"
)

const codeFormMaxBytes = 1024

type codePage struct {
	Error string
}

//go:embed codeui.html
var codeUITemplateSource string

var codeUITemplate = template.Must(template.New("code-ui").Parse(codeUITemplateSource))

func registerCodeRoutes(mux *http.ServeMux, buildID identifierBuilder) {
	mux.HandleFunc("GET /printers/codes", func(w http.ResponseWriter, _ *http.Request) {
		renderCodeUI(w, http.StatusOK, codePage{})
	})
	mux.HandleFunc("POST /printers/codes/generate", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, codeFormMaxBytes)
		if err := r.ParseForm(); err != nil {
			renderCodeUI(w, http.StatusBadRequest, codePage{Error: "form is too large or invalid"})
			return
		}
		kind, err := codeKind(r.FormValue("barcode-kind"))
		if err != nil {
			renderCodeUI(w, http.StatusBadRequest, codePage{Error: err.Error()})
			return
		}
		job := makePrintJobs(1, 1, "", true, false, buildID)[0]
		code, err := newSVGBarcode(job.CodeForBarcode, kind)
		if err != nil {
			renderCodeUI(w, http.StatusInternalServerError, codePage{Error: fmt.Sprintf("Generate code: %v", err)})
			return
		}

		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Disposition", `inline; filename="code.svg"`)
		if err := writeBarcodeSVG(w, code); err != nil {
			return
		}
	})
}

func codeKind(value string) (barcodeKind, error) {
	kind, err := newBarcodeKind(value)
	if err != nil {
		return "", err
	}
	if kind == barcodeKindCode128 {
		return "", fmt.Errorf("unrecognized barcodeKind: %s", value)
	}
	return kind, nil
}

func newSVGBarcode(content string, kind barcodeKind) (barcode.Barcode, error) {
	switch kind {
	case barcodeKindQR:
		return qr.Encode(content, qr.M, qr.Auto)
	case barcodeKindDatamatrix:
		return datamatrix.Encode(content)
	case barcodeKindCode128:
		return nil, fmt.Errorf("unrecognized barcodeKind: %s", kind)
	default:
		return nil, fmt.Errorf("unrecognized barcodeKind: %s", kind)
	}
}

func writeBarcodeSVG(output io.Writer, code barcode.Barcode) error {
	bounds := code.Bounds()
	const quietZone = 4
	if _, err := fmt.Fprintf(output, `<?xml version="1.0" encoding="UTF-8"?><svg xmlns="http://www.w3.org/2000/svg" viewBox="%d %d %d %d" shape-rendering="crispEdges"><rect x="%d" y="%d" width="%d" height="%d" fill="white"/>`, -quietZone, -quietZone, bounds.Dx()+quietZone*2, bounds.Dy()+quietZone*2, -quietZone, -quietZone, bounds.Dx()+quietZone*2, bounds.Dy()+quietZone*2); err != nil {
		return err
	}
	active := []barcodeRect{}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		runs := []barcodeRect{}
		for x := bounds.Min.X; x < bounds.Max.X; {
			if !barcodePixelIsBlack(code, x, y) {
				x++
				continue
			}
			start := x
			for x < bounds.Max.X && barcodePixelIsBlack(code, x, y) {
				x++
			}
			runs = append(runs, barcodeRect{x: start - bounds.Min.X, y: y - bounds.Min.Y, width: x - start, height: 1})
		}

		next := make([]barcodeRect, 0, len(runs))
		for _, run := range runs {
			match := -1
			for index, rectangle := range active {
				if rectangle.x == run.x && rectangle.width == run.width {
					match = index
					break
				}
			}
			if match == -1 {
				next = append(next, run)
				continue
			}
			run = active[match]
			run.height++
			next = append(next, run)
			active = append(active[:match], active[match+1:]...)
		}
		for _, rectangle := range active {
			if err := writeSVGRect(output, rectangle); err != nil {
				return err
			}
		}
		active = next
	}
	for _, rectangle := range active {
		if err := writeSVGRect(output, rectangle); err != nil {
			return err
		}
	}
	_, err := io.WriteString(output, "</svg>")
	return err
}

type barcodeRect struct {
	x      int
	y      int
	width  int
	height int
}

func writeSVGRect(output io.Writer, rectangle barcodeRect) error {
	_, err := fmt.Fprintf(output, `<rect x="%d" y="%d" width="%d" height="%d"/>`, rectangle.x, rectangle.y, rectangle.width, rectangle.height)
	return err
}

func barcodePixelIsBlack(code image.Image, x int, y int) bool {
	return color.GrayModel.Convert(code.At(x, y)).(color.Gray).Y < 128
}

func renderCodeUI(w http.ResponseWriter, status int, page codePage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = codeUITemplate.Execute(w, page)
}
