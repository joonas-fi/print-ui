package ippprint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenPrinting/goipp"
)

func TestPrintPDF(t *testing.T) {
	const pdf = "%PDF-1.7\ncontents"
	var uri string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/printers/test?x=1" || r.Header.Get("Content-Type") != goipp.ContentType || r.Header.Get("Accept") != goipp.ContentType {
			t.Errorf("unexpected request: %s %s %v", r.Method, r.URL, r.Header)
		}
		var msg goipp.Message
		if err := msg.Decode(r.Body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if msg.Code != goipp.Code(goipp.OpPrintJob) {
			t.Errorf("operation = %v", msg.Code)
		}
		wantOps := goipp.Attributes{
			goipp.MakeAttr("attributes-charset", goipp.TagCharset, goipp.String("utf-8")),
			goipp.MakeAttr("attributes-natural-language", goipp.TagLanguage, goipp.String("en")),
			goipp.MakeAttr("printer-uri", goipp.TagURI, goipp.String(uri)),
			goipp.MakeAttr("job-name", goipp.TagName, goipp.String("Test document")),
			goipp.MakeAttr("document-format", goipp.TagMimeType, goipp.String("application/pdf")),
		}
		wantJob := goipp.Attributes{
			goipp.MakeAttr("copies", goipp.TagInteger, goipp.Integer(3)),
			goipp.MakeAttr("sides", goipp.TagKeyword, goipp.String("two-sided-long-edge")),
		}
		if !msg.Operation.Equal(wantOps) || !msg.Job.Equal(wantJob) {
			t.Errorf("unexpected attributes: operation=%v job=%v", msg.Operation, msg.Job)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil || string(data) != pdf {
			t.Errorf("document = %q, error %v", data, err)
		}
		response := goipp.NewResponse(goipp.DefaultVersion, goipp.StatusOk, msg.RequestID)
		response.Job.Add(goipp.MakeAttr("job-id", goipp.TagInteger, goipp.Integer(42)))
		if err := response.Encode(w); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()
	uri = "ipp" + strings.TrimPrefix(server.URL, "http") + "/printers/test?x=1"
	copies, sides, name := 3, "two-sided-long-edge", "Test document"
	id, err := PrintPDF(context.Background(), server.Client(), uri, strings.NewReader(pdf), Options{Copies: &copies, Sides: &sides, JobName: &name})
	if err != nil || id == nil || *id != 42 {
		t.Fatalf("job ID = %v, error = %v", id, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestIPPToHTTP(t *testing.T) {
	for _, tc := range []struct{ uri, want string }{
		{"ipp://printer/ipp/print?foo=bar", "http://printer:631/ipp/print?foo=bar"},
		{"ipps://[::1]/ipp/print", "https://[::1]:631/ipp/print"},
		{"ipp://printer:80/print", "http://printer:80/print"},
		{"ipps://printer:123/print", "https://printer:123/print"},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			got, err := ippToHTTP(tc.uri)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, error %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, uri := range []string{"http://printer", "ipp:///missing", "ipp://user@printer", "ipp://printer/#fragment", "ipp://printer:bad"} {
		if _, err := ippToHTTP(uri); err == nil {
			t.Errorf("expected error for %q", uri)
		}
	}
}

func TestURIAndDefaults(t *testing.T) {
	for _, tc := range []struct{ uri, url string }{
		{"ipp://printer/ipp/print", "http://printer:631/ipp/print"},
		{"ipps://[::1]/ipp/print", "https://[::1]:631/ipp/print"},
		{"ipps://printer:123/print", "https://printer:123/print"},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != tc.url {
					t.Errorf("URL = %s", r.URL)
				}
				var msg goipp.Message
				if err := msg.Decode(r.Body); err != nil {
					t.Fatal(err)
				}
				if len(msg.Job) != 0 {
					t.Errorf("unexpected job attributes: %v", msg.Job)
				}
				response := goipp.NewResponse(goipp.DefaultVersion, goipp.StatusOk, msg.RequestID)
				response.Job.Add(goipp.MakeAttr("job-id", goipp.TagInteger, goipp.Integer(1)))
				data, err := response.EncodeBytes()
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
			})}
			if _, err := PrintPDF(context.Background(), client, tc.uri, strings.NewReader("pdf"), Options{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFailures(t *testing.T) {
	badCopies, badSides := 0, "invalid"
	for _, tc := range []struct {
		uri  string
		opts Options
	}{
		{"http://printer", Options{}}, {"ipp:///missing", Options{}}, {"ipp://user@printer", Options{}},
		{"ipp://printer", Options{Copies: &badCopies}}, {"ipp://printer", Options{Sides: &badSides}},
	} {
		if _, err := PrintPDF(context.Background(), nil, tc.uri, strings.NewReader("pdf"), tc.opts); err == nil {
			t.Errorf("expected error for %+v", tc)
		}
	}
	for _, tc := range []struct {
		name     string
		status   int
		response *goipp.Message
		body     string
	}{
		{"http", 500, nil, "failure"},
		{"malformed", 200, nil, "bad"},
		{"ipp error", 200, goipp.NewResponse(goipp.DefaultVersion, goipp.Status(0x0400), 1), ""},
		{"wrong request ID", 200, goipp.NewResponse(goipp.DefaultVersion, goipp.StatusOk, 2), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := tc.body
				if tc.response != nil {
					data, err := tc.response.EncodeBytes()
					if err != nil {
						t.Fatal(err)
					}
					body = string(data)
				}
				return &http.Response{StatusCode: tc.status, Status: fmt.Sprint(tc.status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			if _, err := PrintPDF(context.Background(), client, "ipp://printer", strings.NewReader("pdf"), Options{}); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := PrintPDF(ctx, nil, "ipp://printer", strings.NewReader("pdf"), Options{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancellation error = %v", err)
	}
}

func TestMissingJobID(t *testing.T) {
	response := goipp.NewResponse(goipp.DefaultVersion, goipp.StatusOk, 1)
	data, err := response.EncodeBytes()
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
	})}
	id, err := PrintPDF(context.Background(), client, "ipp://printer", strings.NewReader("pdf"), Options{})
	if err != nil || id != nil {
		t.Fatalf("job ID = %v, error = %v; want nil, nil", id, err)
	}
}
