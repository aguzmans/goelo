// Package service exposes goelo's stateless rating core over HTTP. It is a thin,
// storage-free boundary for callers that cannot import the Go core directly (e.g.
// long-stocks-advisor in Python): POST /rate takes exactly one core.Request JSON
// and returns one core.Result JSON, using the same validation and math as core.Rate
// and the CLI. The server holds no state — every request is independent — so it is
// safe to run behind any process manager or to scale horizontally.
package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/aguzmans/goelo/core"
)

// ContractVersion is the version of the request/response JSON contract. It is
// returned on every response (header X-Goelo-Contract-Version) and by /healthz so a
// client can detect an incompatible server before trusting a result.
const ContractVersion = "1"

// maxRequestBytes caps a single /rate body. A rating request is tiny; this stops a
// runaway or hostile payload from exhausting memory.
const maxRequestBytes = 1 << 20 // 1 MiB

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler returns the HTTP handler for the rating service. buildVersion is the
// binary version (e.g. the GoReleaser tag) reported by /healthz; pass "dev" when
// unknown. The handler is safe for concurrent use.
func NewHandler(buildVersion string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/rate", rateHandler)
	mux.HandleFunc("/healthz", healthHandler(buildVersion))
	mux.HandleFunc("/", notFoundHandler)
	return withContractVersion(mux)
}

// withContractVersion stamps the contract version on every response so clients can
// pin compatibility even on error responses.
func withContractVersion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Goelo-Contract-Version", ContractVersion)
		next.ServeHTTP(w, r)
	})
}

// rateHandler applies one stateless rating request. It mirrors the CLI's decode
// discipline: unknown fields are rejected and the body must be exactly one JSON
// value, so a malformed integration fails loudly instead of silently mis-rating.
func rateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST /rate")
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	dec.DisallowUnknownFields()
	var req core.Request
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode request: "+err.Error())
		return
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			writeError(w, http.StatusBadRequest, "request must contain exactly one JSON value")
			return
		}
		writeError(w, http.StatusBadRequest, "decode trailing input: "+err.Error())
		return
	}

	result, err := core.Rate(req)
	if err != nil {
		// A bad request (validation, algorithm switch) is the client's fault (400);
		// a well-formed request the math could not resolve (e.g. the volatility
		// solver failing to converge) is unprocessable (422).
		status := http.StatusUnprocessableEntity
		if errors.Is(err, core.ErrInvalidRequest) || errors.Is(err, core.ErrAlgorithmChange) {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// healthHandler reports liveness plus the contract and build versions, so an
// operator or orchestrator can check the service without sending a rating request.
func healthHandler(buildVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "use GET /healthz")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"status":           "ok",
			"contract_version": ContractVersion,
			"version":          buildVersion,
		})
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not found; endpoints are POST /rate and GET /healthz")
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
