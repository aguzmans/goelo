package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/aguzmans/goelo/core"
)

func TestCLIJSONRoundTrip(t *testing.T) {
	input := `{"algo":"elo","current":{"algo":"elo","rating":1500,"n":0},"outcomes":[{"opponent_rating":1500,"correct":true,"severity":"medium"}]}`
	var stdout bytes.Buffer
	if err := run([]string{"rate"}, strings.NewReader(input), &stdout); err != nil {
		t.Fatal(err)
	}
	var result core.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON response: %v\n%s", err, stdout.String())
	}
	if result.State.Rating != 1516 || result.Change != 16 {
		t.Fatalf("unexpected CLI result: %+v", result)
	}
}

func TestCLIReadsRequestFileArgument(t *testing.T) {
	input := `{"algo":"glicko2","current":{"algo":"glicko2","rating":1500,"rd":200,"vol":0.06,"n":0},"periods_elapsed":1,"outcomes":[]}`
	path := t.TempDir() + "/request.json"
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := run([]string{"rate", path}, strings.NewReader(""), &stdout); err != nil {
		t.Fatal(err)
	}
	var result core.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.RD < 200.27 || result.RD > 200.28 {
		t.Fatalf("unexpected aged RD %v", result.RD)
	}
}
