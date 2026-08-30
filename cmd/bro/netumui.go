package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/joonas-fi/bro/pkg/escposraster"
	"github.com/spf13/cobra"
)

const (
	netumTextLimit      = 64 << 10
	netumPreviewMaxRows = 16_384
)

type netumPage struct {
	Text        string
	Notice      string
	Error       string
	PreviewPNGs []template.URL
}

//go:embed netumui.html
var netumUITemplateSource string

var netumUITemplate = template.Must(template.New("netum-ui").Parse(netumUITemplateSource))

func netumUIEntrypoint() *cobra.Command {
	listenAddress := "127.0.0.1:8080"
	options := defaultNetumOptions()
	cmd := &cobra.Command{
		Use:   "netum-ui",
		Short: "Run the Netum receipt printer web UI",
		Args:  cobra.NoArgs,
		Run: cli.RunnerNoArgs(func(ctx context.Context, logger *log.Logger) error {
			return serveNetumUI(ctx, listenAddress, logger, func(ctx context.Context, text string) error {
				return printNetumText(ctx, text, options)
			}, func(text string) ([]template.URL, error) { return previewNetumText(text, options) })
		}),
	}
	cmd.Flags().StringVar(&listenAddress, "listen", listenAddress, "HTTP listen address")
	addNetumFlags(cmd, &options)
	return cmd
}

func serveNetumUI(ctx context.Context, address string, logger *log.Logger, printText func(context.Context, string) error, previewText func(string) ([]template.URL, error)) error {
	server := &http.Server{
		Addr:              address,
		Handler:           newNetumUIHandler(printText, previewText),
		ReadHeaderTimeout: httputils.DefaultReadHeaderTimeout,
	}

	logger.Printf("Netum UI listening on http://%s", server.Addr)
	return httputils.CancelableServer(ctx, server, server.ListenAndServe)
}

func newNetumUIHandler(printText func(context.Context, string) error, previewText func(string) ([]template.URL, error)) http.Handler {
	var printerMu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/printers", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("GET /printers", func(w http.ResponseWriter, r *http.Request) {
		page := netumPage{}
		if r.URL.Query().Has("today") {
			page.Text = finnishDate(time.Now())
		}
		renderNetumUI(w, http.StatusOK, page)
	})
	mux.HandleFunc("POST /printers/print", func(w http.ResponseWriter, r *http.Request) {
		text, err := netumFormText(w, r)
		if err != nil {
			renderNetumUI(w, http.StatusBadRequest, netumPage{Text: text, Error: err.Error()})
			return
		}
		printerMu.Lock()
		err = printText(r.Context(), text)
		printerMu.Unlock()
		if err != nil {
			renderNetumUI(w, http.StatusInternalServerError, netumPage{Text: text, Error: fmt.Sprintf("Print failed: %v", err)})
			return
		}
		renderNetumUI(w, http.StatusOK, netumPage{Text: text, Notice: "Printed successfully."})
	})
	mux.HandleFunc("POST /printers/preview", func(w http.ResponseWriter, r *http.Request) {
		text, err := netumFormText(w, r)
		if err != nil {
			renderNetumUI(w, http.StatusBadRequest, netumPage{Text: text, Error: err.Error()})
			return
		}
		previews, err := previewText(text)
		if err != nil {
			renderNetumUI(w, http.StatusBadRequest, netumPage{Text: text, Error: fmt.Sprintf("Preview failed: %v", err)})
			return
		}
		renderNetumUI(w, http.StatusOK, netumPage{Text: text, PreviewPNGs: previews})
	})
	return mux
}

func netumFormText(w http.ResponseWriter, r *http.Request) (string, error) {
	// Form encoding adds a small amount beyond the textarea's byte limit.
	r.Body = http.MaxBytesReader(w, r.Body, netumTextLimit+1024)
	if err := r.ParseForm(); err != nil {
		return "", errors.New("text is too large or invalid")
	}
	text := r.FormValue("text")
	if strings.TrimSpace(text) == "" {
		return text, errors.New("enter text to print")
	}
	if len(text) > netumTextLimit {
		return text, errors.New("text is too large")
	}
	return text, nil
}

func previewNetumText(text string, options netumOptions) ([]template.URL, error) {
	rasterizer, err := escposraster.New(io.Discard, escposraster.Options{
		WidthDots:  options.widthDots,
		BandHeight: options.bandHeight,
	})
	if err != nil {
		return nil, err
	}
	rows := 0
	var previews []template.URL
	err = renderNetumText(text, options.widthDots, func(img image.Image) error {
		bands, err := rasterizer.RasterImages(img)
		if err != nil {
			return err
		}
		for _, band := range bands {
			rows += band.Bounds().Dy()
			if rows > netumPreviewMaxRows {
				return fmt.Errorf("receipt exceeds the %d-row preview limit", netumPreviewMaxRows)
			}
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, band); err != nil {
				return err
			}
			//nolint:gosec // base64-encoded data poses no injection risk
			previews = append(previews, template.URL("data:image/png;base64,"+base64.StdEncoding.EncodeToString(encoded.Bytes())))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return previews, nil
}

func finnishDate(now time.Time) string {
	weekdays := [...]string{"su", "ma", "ti", "ke", "to", "pe", "la"}
	return fmt.Sprintf("%s %d.%d.", weekdays[now.Weekday()], now.Day(), int(now.Month()))
}

func renderNetumUI(w http.ResponseWriter, status int, page netumPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := netumUITemplate.Execute(w, page); err != nil {
		log.Printf("render Netum UI: %v", err)
	}
}
