# Repository Guide

## Project

This is a Go printer UI with four modes served by `printers server`:

- Receipt mode: `GET /printers`, rasterized and sent directly to the Netum NT-1809DD.
- PDF mode: `GET /printers/pdf`, forwarded to the sibling `cupsd` container through `docker exec ... lpr`.
- Label mode: `GET /printers/labels`, rasterized and sent directly to the Brother P-Touch printer.
- Code mode: `GET /printers/codes`, generates an SVG QR or Data Matrix code.
- `GET /` redirects to receipt mode.

Keep receipt rendering pixel/raster based. Do not replace it with printer-native text, barcode, or font commands.

## Important Files

- `cmd/printers/netum.go`: text layout and direct Netum printing.
- `cmd/printers/netumui.go`: shared HTTP server and receipt routes.
- `cmd/printers/pdfui.go`: PDF upload routes and CUPS delegation.
- `cmd/printers/labelui.go`: Brother label routes and form handling.
- `pkg/escposraster/`: ESC/POS raster encoding and transport primitive.
- `Dockerfile`: builds the Go binary and supplies Docker CLI for PDF mode.

## Commands

Run tests and build without cgo:

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build ./cmd/printers
```

Build the container:

```sh
docker build -t print-ui .
```

The container needs the Netum device, Brother serial device, and Docker socket:

```sh
docker run --rm -p 80:80 \
  --device /dev/usb/lp0 \
  --device /dev/rfcomm0 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  print-ui
```

The CUPS container name is `cupsd`. Keep that contract unless deployment configuration is changed at the same time.

## Changes

- Keep `/printers` as the default mode and retain navigation between receipt, PDF, and label modes on all pages.
- Add focused handler tests for new routes and printer behavior; inject external process or printer operations in tests.
- Do not require physical printer hardware for tests.
- Keep Docker runtime dependencies minimal. PDF mode needs `docker-cli`; receipt mode needs direct USB device access.
