package main

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/boombuler/barcode"
)

func TestCodeUIHasAllModeNavigation(t *testing.T) {
	handler := newCodeUIHandler(newTestIDBuilder())
	request := httptest.NewRequest(http.MethodGet, "/printers/codes", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	for _, href := range []string{`href="/printers"`, `href="/printers/pdf"`, `href="/printers/labels"`, `href="/printers/codes"`} {
		if !strings.Contains(response.Body.String(), href) {
			t.Fatalf("navigation %q missing: %s", href, response.Body.String())
		}
	}
}

func TestCodeUIGeneratesSVG(t *testing.T) {
	for _, kind := range []string{"qr", "datamatrix"} {
		t.Run(kind, func(t *testing.T) {
			handler := newCodeUIHandler(newTestIDBuilder())
			form := url.Values{"barcode-kind": {kind}}
			request := httptest.NewRequest(http.MethodPost, "/printers/codes/generate", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "image/svg+xml" {
				t.Fatalf("content type = %q", contentType)
			}
			if !strings.Contains(response.Body.String(), "<svg") || !strings.Contains(response.Body.String(), "<rect") {
				t.Fatalf("SVG content missing: %s", response.Body.String())
			}
			var document struct {
				XMLName xml.Name `xml:"svg"`
			}
			if err := xml.Unmarshal(response.Body.Bytes(), &document); err != nil {
				t.Fatalf("SVG is invalid: %v", err)
			}
		})
	}
}

func TestCodeUIRejectsUnsupportedCodeType(t *testing.T) {
	handler := newCodeUIHandler(newTestIDBuilder())
	request := httptest.NewRequest(http.MethodPost, "/printers/codes/generate", strings.NewReader("barcode-kind=code128"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
}

func TestBarcodeSVGCombinesMatchingVerticalRuns(t *testing.T) {
	imageData := image.NewGray(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			imageData.SetGray(x, y, color.Gray{Y: 0})
		}
	}
	var output bytes.Buffer
	if err := writeBarcodeSVG(&output, testBarcode{Image: imageData}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `<rect x="0" y="0" width="2" height="2"/>`) {
		t.Fatalf("vertical run was not combined: %s", output.String())
	}
}

func newCodeUIHandler(buildID identifierBuilder) http.Handler {
	mux := http.NewServeMux()
	registerCodeRoutes(mux, buildID)
	return mux
}

type testBarcode struct {
	image.Image
}

func (testBarcode) Metadata() barcode.Metadata { return barcode.Metadata{} }

func (testBarcode) Content() string { return "" }
