# Build stage. Base images are digest-pinned; go.sum pins every module checksum
# and `go mod verify` re-checks the downloaded sources against it.
FROM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -mod=readonly -ldflags="-s -w" -o /disclosure-service .

# Run stage: static binary on a distroless base, as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a
COPY --from=build /disclosure-service /disclosure-service
EXPOSE 7599
# The binary defaults to loopback. Pass --listen 0.0.0.0:7599 so the port is
# reachable through the container's published port; the startup warning is
# expected there. Mount the token file and pass --access-token-file for an
# authenticated upstream.
ENTRYPOINT ["/disclosure-service"]
