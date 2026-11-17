package main

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
)

const pdfMaxBytes = 64 << 20

type pdfPage struct {
	Copies      int
	DoubleSided bool
	Notice      string
	Error       string
}

type pdfPrintFunc func(context.Context, io.Reader, int, bool) error

//go:embed pdfui.html
var pdfUITemplateSource string

var pdfUITemplate = template.Must(template.New("pdf-ui").Parse(pdfUITemplateSource))

func registerPDFRoutes(mux *http.ServeMux, printPDF pdfPrintFunc) {
	mux.HandleFunc("GET /printers/pdf", func(w http.ResponseWriter, _ *http.Request) {
		renderPDFUI(w, http.StatusOK, pdfPage{Copies: 1})
	})
	mux.HandleFunc("POST /printers/pdf/print", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, pdfMaxBytes)
		//nolint:gosec // not unbounded so no resource exhaustion
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			renderPDFUI(w, http.StatusBadRequest, pdfPage{Copies: 1, Error: "Upload a PDF smaller than 64 MiB"})
			return
		}
		copies, err := strconv.Atoi(r.FormValue("copies"))
		if err != nil || copies < 1 {
			renderPDFUI(w, http.StatusBadRequest, pdfPage{Copies: 1, Error: "Copies must be a positive number"})
			return
		}
		doubleSided := r.FormValue("double-sided") == "y"
		file, _, err := r.FormFile("uploadedFile")
		if err != nil {
			renderPDFUI(w, http.StatusBadRequest, pdfPage{Copies: copies, DoubleSided: doubleSided, Error: "Choose a PDF file"})
			return
		}
		defer file.Close()
		if err := printPDF(r.Context(), file, copies, doubleSided); err != nil {
			renderPDFUI(w, http.StatusInternalServerError, pdfPage{Copies: copies, DoubleSided: doubleSided, Error: fmt.Sprintf("Print failed: %v", err)})
			return
		}
		renderPDFUI(w, http.StatusOK, pdfPage{Copies: copies, DoubleSided: doubleSided, Notice: "PDF job submitted; printing may not be complete."})
	})
}

func renderPDFUI(w http.ResponseWriter, status int, page pdfPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := pdfUITemplate.Execute(w, page); err != nil {
		log.Printf("render PDF UI: %v", err)
	}
}
