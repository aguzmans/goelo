package core

import (
	"fmt"
	"math"
)

// Rate applies a stateless rating update. ELO batches are processed in request
// order to preserve the existing single-outcome formula. Glicko-2 outcomes are
// one rating period and are processed together.
func Rate(req Request) (Result, error) {
	if err := req.Validate(); err != nil {
		return Result{}, err
	}
	state := initialState(req.Algo, req.Params)
	if req.Current != nil {
		state = *req.Current
	}
	if req.Algo == ELO {
		return rateELO(state, req.Outcomes), nil
	}
	return rateGlicko2(state, req.Outcomes, req.PeriodsElapsed, req.Params)
}

func rateELO(state State, outcomes []Outcome) Result {
	changeTotal := 0.0
	for _, o := range outcomes {
		k := 32.0
		switch o.Severity {
		case "critical":
			k = 64
		case "high":
			k = 48
		case "medium":
			k = 32
		case "low":
			k = 16
		}
		expected := 1 / (1 + math.Pow(10, (o.OpponentRating-state.Rating)/400))
		actual := 0.0
		if *o.Correct {
			actual = 1
		}
		change := math.Round(k * (actual - expected))
		changeTotal += change
		state.Rating += change
		if state.Rating < 800 {
			state.Rating = 800
		}
		if state.Rating > 2400 {
			state.Rating = 2400
		}
		state.N++
	}
	return Result{State: state, Rating: state.Rating, Conservative: state.Rating, Change: changeTotal}
}

func rateGlicko2(state State, outcomes []Outcome, periods int, params Params) (Result, error) {
	tau := params.Tau
	if tau == 0 {
		tau = DefaultTau
	}
	mu := (state.Rating - DefaultRating) / Scale
	phi := state.RD / Scale
	oldRating := state.Rating
	phi = math.Sqrt(phi*phi + float64(periods)*state.Vol*state.Vol)
	rdAfterPreAge := phi * Scale
	diagnostics := Diagnostics{RDAfterPreAge: rdAfterPreAge}
	if len(outcomes) == 0 {
		state.RD = rdAfterPreAge
		return Result{State: state, Rating: state.Rating, RD: state.RD,
			Conservative: state.Rating - 2*state.RD, Change: 0, Diagnostics: diagnostics}, nil
	}

	sum := 0.0
	information := 0.0
	expected := make([]float64, 0, len(outcomes))
	for _, o := range outcomes {
		oppMu := (o.OpponentRating - DefaultRating) / Scale
		oppPhi := o.OpponentRD / Scale
		g := 1 / math.Sqrt(1+3*oppPhi*oppPhi/(math.Pi*math.Pi))
		e := 1 / (1 + math.Exp(-g*(mu-oppMu)))
		expected = append(expected, e)
		information += g * g * e * (1 - e)
		sum += g * (*o.Score - e)
	}
	if information <= 0 || !finite(information) {
		return Result{}, fmt.Errorf("Glicko-2 period has invalid information")
	}
	v := 1 / information
	delta := v * sum
	newVol, err := volatility(phi, state.Vol, v, delta, tau)
	if err != nil {
		return Result{}, err
	}
	phiStar := math.Sqrt(phi*phi + newVol*newVol)
	newPhi := 1 / math.Sqrt(1/(phiStar*phiStar)+1/v)
	newMu := mu + newPhi*newPhi*sum

	state.Rating = Scale*newMu + DefaultRating
	state.RD = Scale * newPhi
	state.Vol = newVol
	state.N += len(outcomes)
	diagnostics.V = v
	diagnostics.Delta = delta
	diagnostics.Expected = expected
	return Result{State: state, Rating: state.Rating, RD: state.RD,
		Conservative: state.Rating - 2*state.RD, Change: state.Rating - oldRating,
		Diagnostics: diagnostics}, nil
}

// volatility solves Glicko-2's Illinois root-finding problem.
func volatility(phi, sigma, v, delta, tau float64) (float64, error) {
	a := math.Log(sigma * sigma)
	f := func(x float64) float64 {
		ex := math.Exp(x)
		den := 2 * math.Pow(phi*phi+v+ex, 2)
		return ex*(delta*delta-phi*phi-v-ex)/den - (x-a)/(tau*tau)
	}
	A := a
	var B float64
	if delta*delta > phi*phi+v {
		B = math.Log(delta*delta - phi*phi - v)
	} else {
		k := 1.0
		for f(a-k*tau) < 0 {
			k++
			if k > 10000 {
				return 0, fmt.Errorf("Glicko-2 volatility solver failed to bracket root")
			}
		}
		B = a - k*tau
	}
	fA, fB := f(A), f(B)
	for i := 0; i < 1000 && math.Abs(B-A) > 1e-6; i++ {
		C := A + (A-B)*fA/(fB-fA)
		fC := f(C)
		if fC*fB <= 0 {
			A, fA = B, fB
		} else {
			fA /= 2
		}
		B, fB = C, fC
	}
	if math.Abs(B-A) > 1e-6 {
		return 0, fmt.Errorf("Glicko-2 volatility solver did not converge")
	}
	return math.Exp(A / 2), nil
}
