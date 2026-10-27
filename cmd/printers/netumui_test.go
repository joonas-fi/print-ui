package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func noPDF(context.Context, io.Reader, int, bool) error { return nil }

func TestFinnishDate(t *testing.T) {
	got := finnishDate(time.Date(2026, time.August, 29, 12, 0, 0, 0, time.Local))
	if got != "la 29.8." {
		t.Fatalf("date = %q", got)
	}
}

func TestNetumUIPrintsText(t *testing.T) {
	var printed string
	handler := newNetumUIHandler(func(_ context.Context, text string) error {
		printed = text
		return nil
	}, func(string) ([]string, error) { return nil, nil }, noPDF)
	form := url.Values{"text": {"hello\nworld"}}
	req := httptest.NewRequest(http.MethodPost, "/printers/print", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if printed != "hello\nworld" {
		t.Fatalf("printed = %q", printed)
	}
	if !strings.Contains(response.Body.String(), "hello\nworld") {
		t.Fatalf("printed text was not retained: %s", response.Body.String())
	}
}

func TestNetumUIRejectsBlankText(t *testing.T) {
	handler := newNetumUIHandler(func(context.Context, string) error {
		t.Fatal("printer should not be called")
		return nil
	}, func(string) ([]string, error) { return nil, nil }, noPDF)
	req := httptest.NewRequest(http.MethodPost, "/printers/print", strings.NewReader("text=+++"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestNetumUIPreviewsWithoutPrinting(t *testing.T) {
	handler := newNetumUIHandler(func(context.Context, string) error {
		t.Fatal("printer should not be called")
		return nil
	}, func(text string) ([]string, error) {
		if text != "hello" {
			t.Fatalf("preview text = %q", text)
		}
		return []string{"preview"}, nil
	}, noPDF)
	form := url.Values{"text": {"hello"}}
	req := httptest.NewRequest(http.MethodPost, "/printers/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `src="data:image/png;base64,preview"`) {
		t.Fatalf("preview image missing: %s", response.Body.String())
	}
}

func TestNetumUIRootRedirectsToPrinters(t *testing.T) {
	handler := newNetumUIHandler(func(context.Context, string) error { return nil }, func(string) ([]string, error) { return nil, nil }, noPDF)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d", response.Code)
	}
	if location := response.Header().Get("Location"); location != "/printers" {
		t.Fatalf("location = %q", location)
	}
}

func TestNetumUIHasLabelNavigation(t *testing.T) {
	handler := newNetumUIHandler(func(context.Context, string) error { return nil }, func(string) ([]string, error) { return nil, nil }, noPDF)

	request := httptest.NewRequest(http.MethodGet, "/printers", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `href="/printers/labels"`) {
		t.Fatalf("label navigation missing: %s", response.Body.String())
	}
}

func TestPreviewNetumTextProducesRasterPNGs(t *testing.T) {
	previews, err := previewNetumText("hello", defaultNetumOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) == 0 || !strings.HasPrefix(previews[0], "iVBOR") {
		t.Fatalf("previews = %v", previews)
	}
}
