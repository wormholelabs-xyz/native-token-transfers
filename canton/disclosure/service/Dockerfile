# Build stage. Base images are digest-pinned; go.sum pins every module checksum
# and `go mod verify` re-checks the downloaded sources against it.
FROM golang:1.26.11-alpine@sha256:ecb67f987aa1a57794c91d19ade7ae0209f2f0f5496aaef9e6ae85ddf92d02ab AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -mod=readonly -ldflags="-s -w" -o /disclosure-service .

# Run stage: static binary on a distroless base, as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a
COPY --from=build /disclosure-service /disclosure-service
EXPOSE 7599
# The binary defaults to loopback. Put an edge proxy (TLS, rate limit, WAF)
# in front. Pass --listen 0.0.0.0:7599 --behind-edge-proxy so the proxy
# reaches the port. Mount the token file read-only and pass
# --access-token-file for an authenticated upstream.
ENTRYPOINT ["/disclosure-service"]
