package service

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aguzmans/goelo/core"
)

func do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	NewHandler("test-build").ServeHTTP(rr, req)
	return rr
}

func TestRate_ELO(t *testing.T) {
	body := `{"algo":"elo","current":{"algo":"elo","rating":1500,"n":12},"outcomes":[{"opponent_rating":1660,"correct":true,"severity":"high"}]}`
	rr := do(t, http.MethodPost, "/rate", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("X-Goelo-Contract-Version"); got != ContractVersion {
		t.Errorf("contract-version header = %q, want %q", got, ContractVersion)
	}
	var res core.Result
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	// Same numbers the CLI and core produce for this request.
	if res.State.Rating != 1534 || res.Change != 34 {
		t.Fatalf("rating=%v change=%v, want 1534/34", res.State.Rating, res.Change)
	}
}

func TestRate_Glicko2(t *testing.T) {
	body := `{"algo":"glicko2","current":{"algo":"glicko2","rating":1500,"rd":350,"vol":0.06,"n":0},"outcomes":[{"opponent_rating":1500,"opponent_rd":350,"score":1.0}],"params":{"tau":0.5}}`
	rr := do(t, http.MethodPost, "/rate", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var res core.Result
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if math.Abs(res.Rating-1662.31) > 0.1 {
		t.Fatalf("rating = %v, want ~1662.31", res.Rating)
	}
	if res.RD <= 0 || res.RD >= 350 {
		t.Fatalf("RD = %v, want a shrunk positive value < 350", res.RD)
	}
}

func TestRate_Errors(t *testing.T) {
	cases := []struct {
		name, method, body string
		want               int
	}{
		{"bad algo", http.MethodPost, `{"algo":"bogus","outcomes":[]}`, http.StatusBadRequest},
		{"algorithm switch", http.MethodPost, `{"algo":"glicko2","current":{"algo":"elo","rating":1500,"n":1},"outcomes":[{"opponent_rating":1500,"opponent_rd":50,"score":1}]}`, http.StatusBadRequest},
		{"elo missing correct", http.MethodPost, `{"algo":"elo","outcomes":[{"opponent_rating":1500}]}`, http.StatusBadRequest},
		{"malformed json", http.MethodPost, `{"algo":`, http.StatusBadRequest},
		{"unknown field", http.MethodPost, `{"algo":"elo","surprise":1,"outcomes":[]}`, http.StatusBadRequest},
		{"trailing data", http.MethodPost, `{"algo":"elo","outcomes":[]}{"x":1}`, http.StatusBadRequest},
		{"wrong method", http.MethodGet, "", http.StatusMethodNotAllowed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := do(t, c.method, "/rate", c.body)
			if rr.Code != c.want {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, c.want, rr.Body.String())
			}
			var er errorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &er); err != nil || er.Error == "" {
				t.Fatalf("expected a JSON error body, got: %s", rr.Body.String())
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	rr := do(t, http.MethodGet, "/healthz", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var h map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &h); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if h["status"] != "ok" || h["contract_version"] != ContractVersion || h["version"] != "test-build" {
		t.Fatalf("unexpected health body: %v", h)
	}
}

func TestNotFound(t *testing.T) {
	rr := do(t, http.MethodGet, "/nope", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
	// Even error responses carry the contract version.
	if rr.Header().Get("X-Goelo-Contract-Version") != ContractVersion {
		t.Error("missing contract-version header on 404")
	}
}
