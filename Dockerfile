# Also update GitHub Actions workflow when bumping
FROM docker.io/library/golang:1.18 AS builder

WORKDIR /src/
COPY . .
RUN go install github.com/nishanths/exhaustive/...@latest
RUN exhaustive ./...
RUN CGO_ENABLED=0 go build -ldflags='-extldflags=-static' -o /bin/delic ./cmd

FROM docker.io/library/alpine:latest
COPY --from=builder /bin/delic /delic/bin/delic
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /delic/bin
ENTRYPOINT ["./delic"]
