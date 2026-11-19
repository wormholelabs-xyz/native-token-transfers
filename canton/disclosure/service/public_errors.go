package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
)

// maxLoggedUpstreamBodyBytes bounds the upstream error body excerpt kept for the log.
const maxLoggedUpstreamBodyBytes = 512

// maxEchoedClientValueBytes bounds each client value that an error message repeats.
const maxEchoedClientValueBytes = 32

// publicErrorBody is the only error shape a client sees.
type publicErrorBody struct {
	Error     string `json:"error"`
	RequestID string `json:"requestId"`
}

// upstreamHTTPError is a non-200 upstream answer. bodyExcerpt goes to the log only.
type upstreamHTTPError struct {
	operation   string
	statusCode  int
	bodyExcerpt []byte
}

func (e *upstreamHTTPError) Error() string {
	return fmt.Sprintf("disclosure-service: %s: HTTP %d: %q", e.operation, e.statusCode, e.bodyExcerpt)
}

// publicMessageUpstreamListLimit is the fixed client text when the participant refuses a list
// above its http-list-max-elements-limit.
const publicMessageUpstreamListLimit = "upstream ledger list limit reached"

// publicMessageFor is the fixed client text for a 5xx status and its cause.
func publicMessageFor(status int, err error) string {
	if status == http.StatusBadGateway && errors.Is(err, errUpstreamListLimitExceeded) {
		return publicMessageUpstreamListLimit
	}
	return publicMessageForStatus(status)
}

// publicMessageForStatus is the fixed client text for a 5xx status.
func publicMessageForStatus(status int) string {
	switch status {
	case http.StatusBadGateway:
		return "upstream ledger error"
	case http.StatusServiceUnavailable:
		return "service busy, retry"
	case http.StatusGatewayTimeout:
		return "upstream timeout"
	default:
		return "internal error"
	}
}

// newRequestID returns 16 random bytes as 32 lowercase hex characters.
func newRequestID() string {
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		panic(fmt.Sprintf("disclosure-service: crypto/rand failed: %v", err))
	}
	requestID := hex.EncodeToString(idBytes[:])
	if len(requestID) != 32 {
		panic("disclosure-service: request id is not 32 hex characters")
	}
	return requestID
}

// truncateForError bounds a client value before an error message repeats it.
func truncateForError(clientValue string) string {
	if len(clientValue) <= maxEchoedClientValueBytes {
		return clientValue
	}
	return clientValue[:maxEchoedClientValueBytes] + "..."
}

// writePublicError logs err with a new request id, then writes the fixed message for status.
//
// SECURITY: err can hold upstream bodies, URLs, IP addresses, and file paths. Only the log sees it.
// The path and err are quoted: a client value cannot start a new log line.
func (s *server) writePublicError(w http.ResponseWriter, r *http.Request, status int, err error) {
	if status < 500 || status > 599 {
		panic(fmt.Sprintf("disclosure-service: writePublicError needs a 5xx status, got %d", status))
	}
	requestID := newRequestID()
	s.logger.Printf("disclosure-service: request %s %q: status %d: %q", requestID, r.URL.Path, status, err.Error())
	w.Header().Set("X-Request-Id", requestID)
	writeJSON(w, status, publicErrorBody{Error: publicMessageFor(status, err), RequestID: requestID})
}

// writeFlowError writes a 4xx flowError with its own message; every other error is a fixed 5xx.
func (s *server) writeFlowError(w http.ResponseWriter, r *http.Request, err error) {
	var flowErr *flowError
	if !errors.As(err, &flowErr) {
		s.writePublicError(w, r, http.StatusInternalServerError, err)
		return
	}
	if flowErr.status >= 500 {
		s.writePublicError(w, r, flowErr.status, err)
		return
	}
	if flowErr.status < 400 {
		s.writePublicError(w, r, http.StatusInternalServerError, fmt.Errorf("flowError with non-error status %d: %w", flowErr.status, err))
		return
	}
	requestID := newRequestID()
	w.Header().Set("X-Request-Id", requestID)
	writeJSON(w, flowErr.status, publicErrorBody{Error: flowErr.msg, RequestID: requestID})
}
