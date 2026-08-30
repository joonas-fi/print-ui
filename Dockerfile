FROM golang:1.24-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/printers ./cmd/printers

FROM alpine:3.21

# PDF printing delegates to the sibling CUPS container through docker exec.
RUN apk add --no-cache docker-cli

COPY --from=build /out/printers /printers

EXPOSE 80

ENTRYPOINT ["/printers"]
CMD ["server", "--listen=:80"]
