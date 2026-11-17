package main

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPDFUIPrintsViaInjectedAdapter(t *testing.T) {
	var gotPDF string
	var gotCopies int
	var gotDoubleSided bool
	handler := newNetumUIHandler(
		func(context.Context, string) error { return nil },
		func(string) ([]string, error) { return nil, nil },
		func(_ context.Context, document io.Reader, copies int, doubleSided bool) error {
			data, err := io.ReadAll(document)
			if err != nil {
				return err
			}
			gotPDF = string(data)
			gotCopies = copies
			gotDoubleSided = doubleSided
			return nil
		},
	)
	body, contentType := pdfMultipartBody(t, "receipt.pdf", "%PDF-test", "3", true)
	request := httptest.NewRequest(http.MethodPost, "/printers/pdf/print", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", response.Code, response.Body.String())
	}
	if gotPDF != "%PDF-test" || gotCopies != 3 || !gotDoubleSided {
		t.Fatalf("print arguments = %q, %d, %t", gotPDF, gotCopies, gotDoubleSided)
	}
}

func TestPDFUIRejectsInvalidCopies(t *testing.T) {
	handler := newNetumUIHandler(
		func(context.Context, string) error { return nil },
		func(string) ([]string, error) { return nil, nil },
		func(context.Context, io.Reader, int, bool) error {
			t.Fatal("PDF printer should not be called")
			return nil
		},
	)
	body, contentType := pdfMultipartBody(t, "receipt.pdf", "%PDF-test", "0", false)
	request := httptest.NewRequest(http.MethodPost, "/printers/pdf/print", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Copies must be a positive number") {
		t.Fatalf("validation message missing: %s", response.Body.String())
	}
}

func TestPDFUIHasReceiptNavigation(t *testing.T) {
	handler := newNetumUIHandler(func(context.Context, string) error { return nil }, func(string) ([]string, error) { return nil, nil }, noPDF)
	request := httptest.NewRequest(http.MethodGet, "/printers/pdf", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `href="/printers"`) {
		t.Fatalf("receipt navigation missing: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `href="/printers/labels"`) {
		t.Fatalf("label navigation missing: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `href="/printers/codes"`) {
		t.Fatalf("code navigation missing: %s", response.Body.String())
	}
}

func pdfMultipartBody(t *testing.T, filename string, content string, copies string, doubleSided bool) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("copies", copies); err != nil {
		t.Fatal(err)
	}
	if doubleSided {
		if err := writer.WriteField("double-sided", "y"); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile("uploadedFile", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}
