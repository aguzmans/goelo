package core

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type vectorFile struct {
	Tolerance map[string]float64 `json:"tolerance"`
	Cases     []struct {
		Name              string                     `json:"name"`
		In                json.RawMessage            `json:"in"`
		Out               json.RawMessage            `json:"out"`
		Intermediate      map[string]json.RawMessage `json:"intermediate_checks"`
		ToleranceOverride map[string]float64         `json:"tolerance_override"`
	} `json:"cases"`
}

func readVectors(t *testing.T, path string) vectorFile {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var vf vectorFile
	if err := json.Unmarshal(b, &vf); err != nil {
		t.Fatal(err)
	}
	return vf
}

func TestELOConformanceVectors(t *testing.T) {
	vf := readVectors(t, "vectors/elo_vectors.json")
	for _, tc := range vf.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			var in struct {
				Current  float64 `json:"current"`
				Correct  bool    `json:"correct"`
				Opponent float64 `json:"opponent_rating"`
				Severity string  `json:"severity"`
			}
			var out struct {
				New    float64 `json:"new"`
				Change float64 `json:"change"`
			}
			if err := json.Unmarshal(tc.In, &in); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(tc.Out, &out); err != nil {
				t.Fatal(err)
			}
			r, err := Rate(Request{Algo: ELO, Current: &State{Algo: ELO, Rating: in.Current}, Outcomes: []Outcome{{OpponentRating: in.Opponent, Correct: boolPtr(in.Correct), Severity: in.Severity}}})
			if err != nil {
				t.Fatal(err)
			}
			if r.Rating != out.New || r.Change != out.Change {
				t.Fatalf("got %v/%v, want %v/%v", r.Rating, r.Change, out.New, out.Change)
			}
		})
	}
}

func TestGlicko2ConformanceVectors(t *testing.T) {
	vf := readVectors(t, "vectors/glicko2_vectors.json")
	for _, tc := range vf.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			var in struct {
				Current  *State    `json:"current"`
				Periods  int       `json:"periods_elapsed"`
				Outcomes []Outcome `json:"outcomes"`
			}
			var out struct {
				Rating float64 `json:"rating"`
				RD     float64 `json:"rd"`
				Vol    float64 `json:"vol"`
			}
			if err := json.Unmarshal(tc.In, &in); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(tc.Out, &out); err != nil {
				t.Fatal(err)
			}
			if in.Current != nil {
				in.Current.Algo = Glicko2
			}
			r, err := Rate(Request{Algo: Glicko2, Current: in.Current, PeriodsElapsed: in.Periods, Outcomes: in.Outcomes})
			if err != nil {
				t.Fatal(err)
			}
			tol := vf.Tolerance
			if tc.ToleranceOverride != nil {
				tol = tc.ToleranceOverride
			}
			assertNear(t, r.Rating, out.Rating, tol["rating"])
			assertNear(t, r.RD, out.RD, tol["rd"])
			assertNear(t, r.State.Vol, out.Vol, tol["vol"])
			if raw, ok := tc.Intermediate["rd_after_preage"]; ok {
				var want float64
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatal(err)
				}
				assertNear(t, r.Diagnostics.RDAfterPreAge, want, 0.0001)
			}
			if raw, ok := tc.Intermediate["v"]; ok {
				var want float64
				_ = json.Unmarshal(raw, &want)
				assertNear(t, r.Diagnostics.V, want, 0.001)
			}
			if raw, ok := tc.Intermediate["delta"]; ok {
				var want float64
				_ = json.Unmarshal(raw, &want)
				assertNear(t, r.Diagnostics.Delta, want, 0.001)
			}
			if raw, ok := tc.Intermediate["E"]; ok {
				var want []float64
				if err := json.Unmarshal(raw, &want); err != nil {
					t.Fatal(err)
				}
				if len(r.Diagnostics.Expected) != len(want) {
					t.Fatalf("expected vector length %d, got %d", len(want), len(r.Diagnostics.Expected))
				}
				for i := range want {
					assertNear(t, r.Diagnostics.Expected[i], want[i], 0.001)
				}
			}
		})
	}
}

func assertNear(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("got %.8f, want %.8f (tolerance %.8f)", got, want, tolerance)
	}
}
