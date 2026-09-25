// Example caller-side adapter. The application loads and persists this state
// with MySQL; the rating package itself has no database dependency.
package main

import (
	"fmt"

	"github.com/aguzmans/goelo/core"
)

func main() {
	current := core.State{Algo: core.ELO, Rating: 1500, N: 12}
	correct := true
	result, err := core.Rate(core.Request{
		Algo:    core.ELO,
		Current: &current,
		Outcomes: []core.Outcome{{
			OpponentRating: 1660, // caller computes the cited-source average
			Correct:        &correct,
			Severity:       "high",
		}},
	})
	if err != nil {
		panic(err)
	}

	// Persist result.State in MySQL and write audit/history rows in the caller.
	fmt.Printf("rating=%0.0f change=%0.0f state=%+v\n", result.Rating, result.Change, result.State)
}
