package core

import (
	"math"
	"testing"
)

func boolPtr(v bool) *bool        { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestELOUsesSeverityAndClamps(t *testing.T) {
	cases := []struct {
		name              string
		current, opponent float64
		correct           bool
		severity          string
		want, delta       float64
	}{
		{"even win", 1500, 1500, true, "medium", 1516, 16},
		{"high underdog win", 1500, 1900, true, "high", 1544, 44},
		{"floor", 810, 810, false, "medium", 800, -16},
		{"ceiling", 2390, 2390, true, "medium", 2400, 16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Rate(Request{Algo: ELO, Current: &State{Algo: ELO, Rating: tc.current}, Outcomes: []Outcome{{OpponentRating: tc.opponent, Correct: boolPtr(tc.correct), Severity: tc.severity}}})
			if err != nil {
				t.Fatal(err)
			}
			if r.Rating != tc.want || r.Change != tc.delta {
				t.Fatalf("got rating/change %v/%v, want %v/%v", r.Rating, r.Change, tc.want, tc.delta)
			}
		})
	}
}

func TestGlicko2PaperExample(t *testing.T) {
	r, err := Rate(Request{Algo: Glicko2, Current: &State{Algo: Glicko2, Rating: 1500, RD: 200, Vol: 0.06}, Outcomes: []Outcome{
		{OpponentRating: 1400, OpponentRD: 30, Score: floatPtr(1)},
		{OpponentRating: 1550, OpponentRD: 100, Score: floatPtr(0)},
		{OpponentRating: 1700, OpponentRD: 300, Score: floatPtr(0)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.Rating-1464.06) > 0.1 || math.Abs(r.RD-151.52) > 0.1 || math.Abs(r.State.Vol-0.05999) > 1e-4 {
		t.Fatalf("unexpected paper result: %+v", r)
	}
}

func TestGlicko2PeriodsElapsedAgesBeforeOutcomes(t *testing.T) {
	state := &State{Algo: Glicko2, Rating: 1500, RD: 200, Vol: 0.06}
	outcomes := []Outcome{{OpponentRating: 1400, OpponentRD: 30, Score: floatPtr(1)}, {OpponentRating: 1550, OpponentRD: 100, Score: floatPtr(0)}, {OpponentRating: 1700, OpponentRD: 300, Score: floatPtr(0)}}
	aged, err := Rate(Request{Algo: Glicko2, Current: state, PeriodsElapsed: 1, Outcomes: outcomes})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(aged.Diagnostics.RDAfterPreAge-200.2714) > 0.0001 {
		t.Fatalf("pre-aged RD=%v", aged.Diagnostics.RDAfterPreAge)
	}
	if aged.RD <= 151.52 {
		t.Fatalf("expected aged batch RD > no-gap RD, got %v", aged.RD)
	}
}

func TestGlicko2EmptyBatchOnlyAppliesExplicitAging(t *testing.T) {
	state := &State{Algo: Glicko2, Rating: 1500, RD: 200, Vol: 0.06}
	unchanged, err := Rate(Request{Algo: Glicko2, Current: state})
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.RD != 200 {
		t.Fatalf("periods_elapsed=0 should keep RD, got %v", unchanged.RD)
	}
	aged, err := Rate(Request{Algo: Glicko2, Current: state, PeriodsElapsed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(aged.RD-200.2714) > 0.0001 {
		t.Fatalf("one idle period RD=%v", aged.RD)
	}
}

func TestRejectsAlgorithmSwitch(t *testing.T) {
	_, err := Rate(Request{Algo: Glicko2, Current: &State{Algo: ELO, Rating: 1500}})
	if err != ErrAlgorithmChange {
		t.Fatalf("got %v, want ErrAlgorithmChange", err)
	}
}

func TestRejectsMalformedGlickoOutcome(t *testing.T) {
	_, err := Rate(Request{Algo: Glicko2, Outcomes: []Outcome{{OpponentRating: 1500, OpponentRD: 50}}})
	if err == nil {
		t.Fatal("expected missing score error")
	}
}

func TestRejectsInvalidDefaults(t *testing.T) {
	_, err := Rate(Request{Algo: ELO, Params: Params{DefaultRating: 1500.5}})
	if err == nil {
		t.Fatal("expected fractional ELO default to be rejected")
	}
	_, err = Rate(Request{Algo: Glicko2, Params: Params{DefaultRD: -1}})
	if err == nil {
		t.Fatal("expected negative Glicko-2 default RD to be rejected")
	}
}

func FuzzELOParity(f *testing.F) {
	f.Add(1500, 1500, true, "medium")
	f.Add(1200, 1900, true, "high")
	f.Fuzz(func(t *testing.T, current, opponent int, correct bool, severity string) {
		current = 800 + abs(current%1601)
		current = min(current, 2400)
		opponent = 800 + abs(opponent%1601)
		opponent = min(opponent, 2400)
		r, err := Rate(Request{Algo: ELO, Current: &State{Algo: ELO, Rating: float64(current)}, Outcomes: []Outcome{{OpponentRating: float64(opponent), Correct: boolPtr(correct), Severity: severity}}})
		if err != nil {
			t.Fatal(err)
		}
		k := 32.0
		switch severity {
		case "critical":
			k = 64
		case "high":
			k = 48
		case "medium":
			k = 32
		case "low":
			k = 16
		}
		expected := 1 / (1 + math.Pow(10, float64(opponent-current)/400))
		actual := 0.0
		if correct {
			actual = 1
		}
		change := math.Round(k * (actual - expected))
		want := math.Max(800, math.Min(2400, float64(current)+change))
		if r.Rating != want || r.Change != change {
			t.Fatalf("got %v/%v want %v/%v", r.Rating, r.Change, want, change)
		}
	})
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
