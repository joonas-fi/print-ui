package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLabelUIHasPrintDeskNavigation(t *testing.T) {
	handler := newLabelUIHandler(newTestIDBuilder(), func(context.Context, warehouseOptions) ([]printJob, error) {
		return nil, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/printers/labels", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	for _, href := range []string{`href="/printers"`, `href="/printers/pdf"`, `href="/printers/labels"`} {
		if !strings.Contains(response.Body.String(), href) {
			t.Fatalf("navigation %q missing: %s", href, response.Body.String())
		}
	}
}

func TestLabelUIPrintsWithSubmittedOptions(t *testing.T) {
	var printed warehouseOptions
	handler := newLabelUIHandler(newTestIDBuilder(), func(_ context.Context, options warehouseOptions) ([]printJob, error) {
		printed = options
		return []printJob{{LabelHumanReadable: "id1"}}, nil
	})
	form := labelFormValues()
	request := httptest.NewRequest(http.MethodPost, "/printers/labels/print", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if printed.uniqueLabels != 2 || printed.copies != 3 || printed.barcodeKind != barcodeKindQR || !printed.barcodes || !printed.texts || !printed.feed {
		t.Fatalf("printed options = %#v", printed)
	}
	if !strings.Contains(response.Body.String(), "Printed 1 label(s). IDs: id1") {
		t.Fatalf("success message missing: %s", response.Body.String())
	}
}

func TestLabelUIPreviewsWithoutPrinting(t *testing.T) {
	called := false
	handler := newLabelUIHandler(newTestIDBuilder(), func(context.Context, warehouseOptions) ([]printJob, error) {
		called = true
		return nil, nil
	})
	form := labelFormValues()
	request := httptest.NewRequest(http.MethodPost, "/printers/labels/preview", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if called {
		t.Fatal("preview called printer")
	}
	if !strings.Contains(response.Body.String(), `src="data:image/png;base64,`) {
		t.Fatalf("preview image missing: %s", response.Body.String())
	}
}

func TestLabelUIRejectsInvalidOptions(t *testing.T) {
	handler := newLabelUIHandler(newTestIDBuilder(), func(context.Context, warehouseOptions) ([]printJob, error) {
		t.Fatal("printer should not be called")
		return nil, nil
	})
	request := httptest.NewRequest(http.MethodPost, "/printers/labels/print", strings.NewReader("unique-labels=0&copies=1&barcode-kind=qr&texts=on"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "unique labels must be a positive number") {
		t.Fatalf("validation message missing: %s", response.Body.String())
	}
}

func newLabelUIHandler(buildID identifierBuilder, printLabels labelPrintFunc) http.Handler {
	mux := http.NewServeMux()
	registerLabelRoutes(mux, buildID, printLabels)
	return mux
}

func labelFormValues() url.Values {
	return url.Values{
		"unique-labels": {"2"},
		"copies":        {"3"},
		"barcode-kind":  {"qr"},
		"barcodes":      {"on"},
		"texts":         {"on"},
		"feed":          {"on"},
	}
}
