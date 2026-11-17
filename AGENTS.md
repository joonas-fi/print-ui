# Repository Guide

## Project

This is a Go printer UI with four modes served by `printers server`:

- Receipt mode: `GET /printers`, rasterized and sent directly to the Netum NT-1809DD.
- PDF mode: `GET /printers/pdf`, sent to an IPPS printer.
- Label mode: `GET /printers/labels`, rasterized and sent directly to the Brother P-Touch printer.
- Code mode: `GET /printers/codes`, generates an SVG QR or Data Matrix code.
- `GET /` redirects to receipt mode.

Keep receipt rendering pixel/raster based. Do not replace it with printer-native text, barcode, or font commands.

## Important Files

- `cmd/printers/netum.go`: text layout and direct Netum printing.
- `cmd/printers/netumui.go`: shared HTTP server and receipt routes.
- `cmd/printers/pdfui.go`: PDF upload routes and printing.
- `cmd/printers/labelui.go`: Brother label routes and form handling.
- `pkg/escposraster/`: ESC/POS raster encoding and transport primitive.
- `Dockerfile`: builds the Go binary and supplies CA certificates for PDF mode.

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

The container needs the Netum device, Brother serial device, and network access to the IPPS printer:

```sh
docker run --rm -p 80:80 \
  --device /dev/usb/lp0 \
  --device /dev/rfcomm0 \
  print-ui
```

The default IPPS endpoint is `ipps://brotherlaser.home.fn61.net:443/printers/brotherlaser`. Override it with `server --listen=:80 --ipp-printer-uri=<uri>` after the image name. The printer's TLS certificate must be trusted by the container.

## Changes

- Keep `/printers` as the default mode and retain navigation between receipt, PDF, and label modes on all pages.
- Add focused handler tests for new routes and printer behavior; inject external process or printer operations in tests.
- Do not require physical printer hardware for tests.
- Keep Docker runtime dependencies minimal. PDF mode needs CA certificates and network access; receipt mode needs direct USB device access.
