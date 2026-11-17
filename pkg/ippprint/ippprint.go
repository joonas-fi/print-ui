// Package ippprint submits PDF documents using IPP Print-Job.
package ippprint

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/OpenPrinting/goipp"
)

// Options controls optional IPP job attributes. Nil values use printer defaults.
// Sides may be "one-sided", "two-sided-long-edge", or "two-sided-short-edge".
type Options struct {
	Copies  *int
	Sides   *string
	JobName *string
}

// PrintPDF streams pdf to printerURI and returns the printer-assigned job ID,
// or nil if the successful response does not contain one.
// printerURI must use ipp or ipps. A nil client uses http.DefaultClient.
// The caller retains ownership of pdf. Cancellation cannot interrupt an
// arbitrary reader that is blocked in Read.
func PrintPDF(ctx context.Context, client *http.Client, printerURI string, pdf io.Reader, options Options) (*int, error) {
	endpoint, err := ippToHTTP(printerURI)
	if err != nil {
		return nil, err
	}
	if pdf == nil {
		return nil, fmt.Errorf("PDF reader is nil")
	}
	if options.Copies != nil && (*options.Copies < 1 || int64(*options.Copies) > 2147483647) {
		return nil, fmt.Errorf("copies must be between 1 and 2147483647")
	}
	if options.Sides != nil && *options.Sides != "one-sided" && *options.Sides != "two-sided-long-edge" && *options.Sides != "two-sided-short-edge" {
		return nil, fmt.Errorf("invalid sides value %q", *options.Sides)
	}

	const requestID = 1
	msg := goipp.NewRequest(goipp.DefaultVersion, goipp.OpPrintJob, requestID)
	msg.Operation.Add(goipp.MakeAttribute("attributes-charset", goipp.TagCharset, goipp.String("utf-8")))
	msg.Operation.Add(goipp.MakeAttribute("attributes-natural-language", goipp.TagLanguage, goipp.String("en")))
	msg.Operation.Add(goipp.MakeAttribute("printer-uri", goipp.TagURI, goipp.String(printerURI)))
	if options.JobName != nil {
		msg.Operation.Add(goipp.MakeAttribute("job-name", goipp.TagName, goipp.String(*options.JobName)))
	}
	msg.Operation.Add(goipp.MakeAttribute("document-format", goipp.TagMimeType, goipp.String("application/pdf")))
	if options.Copies != nil {
		msg.Job.Add(goipp.MakeAttribute("copies", goipp.TagInteger, goipp.Integer(*options.Copies)))
	}
	if options.Sides != nil {
		msg.Job.Add(goipp.MakeAttribute("sides", goipp.TagKeyword, goipp.String(*options.Sides)))
	}
	header, err := msg.EncodeBytes()
	if err != nil {
		return nil, fmt.Errorf("encode IPP request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, io.MultiReader(bytes.NewReader(header), pdf))
	if err != nil {
		return nil, fmt.Errorf("create IPP request: %w", err)
	}
	req.Header.Set("Content-Type", goipp.ContentType)
	req.Header.Set("Accept", goipp.ContentType)
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send IPP request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("IPP HTTP response: %s", resp.Status)
	}
	var result goipp.Message
	if err := result.Decode(resp.Body); err != nil {
		return nil, fmt.Errorf("decode IPP response: %w", err)
	}
	if result.RequestID != requestID {
		return nil, fmt.Errorf("IPP response request ID %d does not match %d", result.RequestID, requestID)
	}
	if result.Code >= 0x0100 {
		return nil, fmt.Errorf("IPP print failed: %s (%s)", goipp.Status(result.Code), strconv.FormatUint(uint64(result.Code), 16))
	}
	for _, attr := range result.Job {
		if attr.Name == "job-id" {
			if len(attr.Values) == 1 && attr.Values[0].T == goipp.TagInteger {
				if id, ok := attr.Values[0].V.(goipp.Integer); ok && id > 0 {
					jobID := int(id)
					return &jobID, nil
				}
			}
			return nil, fmt.Errorf("IPP response has invalid job-id")
		}
	}
	return nil, nil
}

func ippToHTTP(printerURI string) (string, error) {
	u, err := url.Parse(printerURI)
	if err != nil || u == nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "ipp" && u.Scheme != "ipps") {
		return "", fmt.Errorf("invalid IPP printer URI %q", printerURI)
	}
	if u.Port() == "" {
		u.Host = net.JoinHostPort(u.Hostname(), "631")
	}
	if u.Scheme == "ipps" {
		u.Scheme = "https"
	} else {
		u.Scheme = "http"
	}
	return u.String(), nil
}
