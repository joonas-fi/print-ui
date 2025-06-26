package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"

	"github.com/function61/gokit/app/cli"
	"github.com/function61/gokit/net/http/httputils"
	"github.com/spf13/cobra"
)

func main() {
	app := &cobra.Command{
		Short: "Print PDF files",
	}

	app.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Runs the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			routes := http.NewServeMux()
			routes.HandleFunc("GET /", uploadFormHandler)
			routes.HandleFunc("POST /upload", fileUploadHandler)

			srv := &http.Server{
				Addr:              ":80",
				Handler:           routes,
				ReadHeaderTimeout: httputils.DefaultReadHeaderTimeout,
			}

			slog.Info("started server", "addr", srv.Addr)

			return httputils.CancelableServer(ctx, srv, srv.ListenAndServe)

		},
	})

	cli.Execute(app)
}

//go:embed upload.html
var uploadFormHTML string

func uploadFormHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, uploadFormHTML)
}

func fileUploadHandler(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("uploadedFile")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	lpr := exec.CommandContext(
		r.Context(),
		"docker",
		"exec",
		"--interactive",
		"cupsd",
		"lpr",
		"-", // take output from stdin
	)
	lpr.Stdin = file
	if output, err := lpr.CombinedOutput(); err != nil {
		http.Error(w, fmt.Errorf("lpr: %w: %s", err, string(output)).Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "Print OK")
}
