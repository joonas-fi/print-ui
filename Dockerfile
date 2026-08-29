FROM golang:1.24-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/bro ./cmd/bro

FROM scratch

COPY --from=build /out/bro /bro

EXPOSE 80

ENTRYPOINT ["/bro"]
CMD ["netum-ui", "--listen=:80"]
