package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
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
		renderPDFUI(w, http.StatusOK, pdfPage{Copies: copies, DoubleSided: doubleSided, Notice: "PDF sent to CUPS"})
	})
}

func printPDFWithCUPS(ctx context.Context, document io.Reader, copies int, doubleSided bool) error {
	if copies < 1 {
		return errors.New("copies must be positive")
	}
	args := []string{"exec", "--interactive", "cupsd", "lpr", "-#", strconv.Itoa(copies)}
	if doubleSided {
		args = append(args, "-o", "sides=two-sided-short-edge")
	}
	args = append(args, "-")
	//nolint:gosec // no user-controlled input
	command := exec.CommandContext(ctx, "docker", args...)
	command.Stdin = document
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("CUPS print: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func renderPDFUI(w http.ResponseWriter, status int, page pdfPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := pdfUITemplate.Execute(w, page); err != nil {
		log.Printf("render PDF UI: %v", err)
	}
}
