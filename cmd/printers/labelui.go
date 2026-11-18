package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const labelFormMaxBytes = 64 << 10

type labelPage struct {
	Values     labelForm
	Notice     string
	Error      string
	PreviewPNG string
}

type labelForm struct {
	UniqueLabels int
	Copies       int
	Reprint      string
	BarcodeKind  string
	Barcodes     bool
	Texts        bool
	Feed         bool
}

type labelPrintFunc func(context.Context, warehouseOptions) ([]printJob, error)

//go:embed labelui.html
var labelUITemplateSource string

var labelUITemplate = template.Must(template.New("label-ui").Parse(labelUITemplateSource))

func registerLabelRoutes(mux *http.ServeMux, buildID identifierBuilder, printLabels labelPrintFunc) {
	var printerMu sync.Mutex
	mux.HandleFunc("GET /printers/labels", func(w http.ResponseWriter, _ *http.Request) {
		renderLabelUI(w, http.StatusOK, labelPage{Values: defaultLabelForm()})
	})
	mux.HandleFunc("POST /printers/labels/print", func(w http.ResponseWriter, r *http.Request) {
		form, err := parseLabelForm(w, r)
		if err != nil {
			renderLabelUI(w, http.StatusBadRequest, labelPage{Values: form, Error: err.Error()})
			return
		}
		options, err := labelWarehouseOptions(form)
		if err != nil {
			renderLabelUI(w, http.StatusBadRequest, labelPage{Values: form, Error: err.Error()})
			return
		}

		printerMu.Lock()
		jobs, err := printLabels(r.Context(), options)
		printerMu.Unlock()
		if err != nil {
			renderLabelUI(w, http.StatusInternalServerError, labelPage{Values: form, Error: fmt.Sprintf("Print failed: %v", err)})
			return
		}
		renderLabelUI(w, http.StatusOK, labelPage{Values: form, Notice: labelPrintNotice(jobs)})
	})
	mux.HandleFunc("POST /printers/labels/preview", func(w http.ResponseWriter, r *http.Request) {
		form, err := parseLabelForm(w, r)
		if err != nil {
			renderLabelUI(w, http.StatusBadRequest, labelPage{Values: form, Error: err.Error()})
			return
		}
		options, err := labelWarehouseOptions(form)
		if err != nil {
			renderLabelUI(w, http.StatusBadRequest, labelPage{Values: form, Error: err.Error()})
			return
		}
		job := makePrintJobs(1, 1, options.reprint, options.barcodes, options.texts, buildID)[0]
		elements, err := defineContent(job.LabelHumanReadable, job.CodeForBarcode, options.barcodeKind)
		var pngData bytes.Buffer
		if err == nil {
			err = render(elements, &pngData)
		}
		if err != nil {
			renderLabelUI(w, http.StatusInternalServerError, labelPage{Values: form, Error: fmt.Sprintf("Render preview: %v", err)})
			return
		}
		renderLabelUI(w, http.StatusOK, labelPage{
			Values:     form,
			PreviewPNG: base64.StdEncoding.EncodeToString(pngData.Bytes()),
		})
	})
}

func defaultLabelForm() labelForm {
	return labelForm{UniqueLabels: 1, Copies: 1, BarcodeKind: string(barcodeKindQR), Barcodes: true, Texts: true, Feed: true}
}

func parseLabelForm(w http.ResponseWriter, r *http.Request) (labelForm, error) {
	r.Body = http.MaxBytesReader(w, r.Body, labelFormMaxBytes)
	if err := r.ParseForm(); err != nil {
		return defaultLabelForm(), errors.New("form is too large or invalid")
	}
	form := labelForm{
		Reprint:     strings.TrimSpace(r.FormValue("reprint")),
		BarcodeKind: r.FormValue("barcode-kind"),
		Barcodes:    r.Form.Has("barcodes"),
		Texts:       r.Form.Has("texts"),
		Feed:        r.Form.Has("feed"),
	}
	var err error
	form.UniqueLabels, err = positiveLabelFormInt(r.FormValue("unique-labels"), "unique labels")
	if err != nil {
		return form, err
	}
	form.Copies, err = positiveLabelFormInt(r.FormValue("copies"), "copies")
	if err != nil {
		return form, err
	}
	if !form.Barcodes && !form.Texts {
		return form, errors.New("barcodes or text must be enabled")
	}
	return form, nil
}

func positiveLabelFormInt(value string, name string) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 {
		return 0, fmt.Errorf("%s must be a positive number", name)
	}
	return number, nil
}

func labelWarehouseOptions(form labelForm) (warehouseOptions, error) {
	kind, err := newBarcodeKind(form.BarcodeKind)
	if err != nil {
		return warehouseOptions{}, err
	}
	return warehouseOptions{
		reprint:      form.Reprint,
		feed:         form.Feed,
		barcodeKind:  kind,
		barcodes:     form.Barcodes,
		texts:        form.Texts,
		uniqueLabels: form.UniqueLabels,
		copies:       form.Copies,
	}, nil
}

func labelPrintNotice(jobs []printJob) string {
	ids := make([]string, 0, len(jobs))
	for _, job := range jobs {
		id := strings.ReplaceAll(job.LabelHumanReadable, "\n", "")
		if id == "" {
			id = job.CodeForBarcode[strings.LastIndex(job.CodeForBarcode, "/")+1:]
		}
		if len(ids) == 0 || ids[len(ids)-1] != id {
			ids = append(ids, id)
		}
	}
	return fmt.Sprintf("Printed %d label(s). IDs: %s", len(jobs), strings.Join(ids, ", "))
}

func renderLabelUI(w http.ResponseWriter, status int, page labelPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := labelUITemplate.Execute(w, page); err != nil {
		log.Printf("render label UI: %v", err)
	}
}
