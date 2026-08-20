# Build stage. go.sum pins and verifies every module checksum.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /disclosure-service .

# Run stage: static binary on a distroless base, as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /disclosure-service /disclosure-service
EXPOSE 7599
# The binary defaults to loopback. Pass --listen 0.0.0.0:7599 so the port is
# reachable through the container's published port; the startup warning is
# expected there. Mount the token file and pass --access-token-file for an
# authenticated upstream.
ENTRYPOINT ["/disclosure-service"]
