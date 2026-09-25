// Package core provides deterministic, storage-free ELO and Glicko-2 rating updates.
package core

import (
	"errors"
	"fmt"
	"math"
)

const (
	DefaultRating = 1500.0
	DefaultRD     = 350.0
	DefaultVol    = 0.06
	DefaultTau    = 0.5
	Scale         = 173.7178
)

type Algorithm string

const (
	ELO     Algorithm = "elo"
	Glicko2 Algorithm = "glicko2"
)

// State is the complete caller-persisted state for an entity. RD and Vol are
// used only by Glicko-2. Timestamps and identity belong to the caller.
type State struct {
	Algo   Algorithm `json:"algo"`
	Rating float64   `json:"rating"`
	RD     float64   `json:"rd,omitempty"`
	Vol    float64   `json:"vol,omitempty"`
	N      int       `json:"n"`
}

type Outcome struct {
	OpponentRating float64  `json:"opponent_rating"`
	OpponentRD     float64  `json:"opponent_rd,omitempty"`
	Score          *float64 `json:"score,omitempty"`
	Correct        *bool    `json:"correct,omitempty"`
	Severity       string   `json:"severity,omitempty"`
}

type Params struct {
	Tau           float64 `json:"tau,omitempty"`
	DefaultRating float64 `json:"default_rating,omitempty"`
	DefaultRD     float64 `json:"default_rd,omitempty"`
	DefaultVol    float64 `json:"default_vol,omitempty"`
}

type Request struct {
	Algo           Algorithm `json:"algo"`
	Current        *State    `json:"current"`
	PeriodsElapsed int       `json:"periods_elapsed,omitempty"`
	Outcomes       []Outcome `json:"outcomes"`
	Params         Params    `json:"params,omitempty"`
}

type Diagnostics struct {
	V             float64   `json:"v,omitempty"`
	Delta         float64   `json:"delta,omitempty"`
	Expected      []float64 `json:"expected,omitempty"`
	RDAfterPreAge float64   `json:"rd_after_preage,omitempty"`
}

type Result struct {
	State        State       `json:"state"`
	Rating       float64     `json:"rating"`
	RD           float64     `json:"rd,omitempty"`
	Conservative float64     `json:"conservative"`
	Change       float64     `json:"change"`
	Diagnostics  Diagnostics `json:"diagnostics,omitempty"`
}

var (
	ErrInvalidRequest  = errors.New("invalid rating request")
	ErrAlgorithmChange = errors.New("rating algorithm cannot be changed for an existing state")
)

func (r Request) Validate() error {
	if r.Algo != ELO && r.Algo != Glicko2 {
		return fmt.Errorf("%w: algo must be %q or %q", ErrInvalidRequest, ELO, Glicko2)
	}
	if r.PeriodsElapsed < 0 {
		return fmt.Errorf("%w: periods_elapsed must be non-negative", ErrInvalidRequest)
	}
	if r.Algo == ELO && r.PeriodsElapsed != 0 {
		return fmt.Errorf("%w: periods_elapsed applies only to Glicko-2", ErrInvalidRequest)
	}
	if r.Params.DefaultRating != 0 && !finite(r.Params.DefaultRating) {
		return fmt.Errorf("%w: default_rating must be finite", ErrInvalidRequest)
	}
	if r.Current != nil {
		if r.Current.Algo != r.Algo {
			return ErrAlgorithmChange
		}
		if err := validateState(*r.Current); err != nil {
			return err
		}
	}
	if r.Algo == ELO && r.Current == nil && r.Params.DefaultRating != 0 && r.Params.DefaultRating != math.Trunc(r.Params.DefaultRating) {
		return fmt.Errorf("%w: ELO default_rating must be an integer", ErrInvalidRequest)
	}
	for i, o := range r.Outcomes {
		if !finite(o.OpponentRating) {
			return fmt.Errorf("%w: outcome %d has invalid opponent_rating", ErrInvalidRequest, i)
		}
		if r.Algo == ELO {
			if o.Correct == nil {
				return fmt.Errorf("%w: ELO outcome %d requires correct", ErrInvalidRequest, i)
			}
		} else {
			if !finite(o.OpponentRD) || o.OpponentRD <= 0 {
				return fmt.Errorf("%w: Glicko-2 outcome %d requires positive opponent_rd", ErrInvalidRequest, i)
			}
			if o.Score == nil || !finite(*o.Score) || *o.Score < 0 || *o.Score > 1 {
				return fmt.Errorf("%w: Glicko-2 outcome %d score must be between 0 and 1", ErrInvalidRequest, i)
			}
		}
	}
	if r.Algo == Glicko2 {
		tau := r.Params.Tau
		if tau != 0 && (!finite(tau) || tau <= 0) {
			return fmt.Errorf("%w: tau must be positive", ErrInvalidRequest)
		}
		if r.Params.DefaultRD != 0 && (!finite(r.Params.DefaultRD) || r.Params.DefaultRD <= 0) {
			return fmt.Errorf("%w: default_rd must be positive", ErrInvalidRequest)
		}
		if r.Params.DefaultVol != 0 && (!finite(r.Params.DefaultVol) || r.Params.DefaultVol <= 0) {
			return fmt.Errorf("%w: default_vol must be positive", ErrInvalidRequest)
		}
	}
	return nil
}

func validateState(s State) error {
	if s.Algo != ELO && s.Algo != Glicko2 {
		return fmt.Errorf("%w: current state has unsupported algo %q", ErrInvalidRequest, s.Algo)
	}
	if !finite(s.Rating) || s.N < 0 {
		return fmt.Errorf("%w: current state has invalid rating or outcome count", ErrInvalidRequest)
	}
	if s.Algo == Glicko2 && (!finite(s.RD) || s.RD <= 0 || !finite(s.Vol) || s.Vol <= 0) {
		return fmt.Errorf("%w: Glicko-2 state requires positive finite rd and vol", ErrInvalidRequest)
	}
	if s.Algo == ELO && s.Rating != math.Trunc(s.Rating) {
		return fmt.Errorf("%w: ELO rating must be an integer", ErrInvalidRequest)
	}
	return nil
}

func initialState(algo Algorithm, p Params) State {
	rating := p.DefaultRating
	if rating == 0 {
		rating = DefaultRating
	}
	s := State{Algo: algo, Rating: rating}
	if algo == Glicko2 {
		s.RD = p.DefaultRD
		if s.RD == 0 {
			s.RD = DefaultRD
		}
		s.Vol = p.DefaultVol
		if s.Vol == 0 {
			s.Vol = DefaultVol
		}
	}
	return s
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
