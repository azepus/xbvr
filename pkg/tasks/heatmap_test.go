package tasks

import (
	"encoding/json"
	"testing"
)

func TestCalculateSpeed_FullStroke(t *testing.T) {
	// Full stroke 0 to 100 in 1 second (1000ms) = 100 units/s
	script := Script{
		Actions: []Action{
			{At: 0, Pos: 0},
			{At: 1000, Pos: 100},
		},
	}
	speed := script.CalculateSpeed()
	if speed != 100 {
		t.Fatalf("expected speed 100 units/s, got %d", speed)
	}
}

func TestCalculateSpeed_FullCycle(t *testing.T) {
	// Full cycle: 0 -> 100 in 500ms, then 100 -> 0 in 500ms (1000ms total, 200 units traveled)
	// Average speed = 200 units / 1s = 200 units/s
	script := Script{
		Actions: []Action{
			{At: 0, Pos: 0},
			{At: 500, Pos: 100},
			{At: 1000, Pos: 0},
		},
	}
	speed := script.CalculateSpeed()
	if speed != 200 {
		t.Fatalf("expected speed 200 units/s, got %d", speed)
	}
}

func TestCalculateSpeed_AmplitudeDifference(t *testing.T) {
	// Compare two scripts with the EXACT same frequency/APM (2 actions in 1 second):
	// Large stroke (0 to 100):
	scriptLarge := Script{
		Actions: []Action{
			{At: 0, Pos: 0},
			{At: 1000, Pos: 100},
		},
	}
	// Small stroke (45 to 55):
	scriptSmall := Script{
		Actions: []Action{
			{At: 0, Pos: 45},
			{At: 1000, Pos: 55},
		},
	}

	speedLarge := scriptLarge.CalculateSpeed()
	speedSmall := scriptSmall.CalculateSpeed()

	if speedLarge != 100 {
		t.Fatalf("expected speedLarge 100, got %d", speedLarge)
	}
	if speedSmall != 10 {
		t.Fatalf("expected speedSmall 10, got %d", speedSmall)
	}
}

func TestCalculateSpeed_IdlePauseHandling(t *testing.T) {
	// Action from 0 to 100 in 500ms (speed = 200 units/s)
	// Followed by a 60-second pause at pos 100
	// Then action from 100 to 0 in 500ms (speed = 200 units/s)
	// The active speed should remain 200 units/s, not diluted by the 60s pause
	script := Script{
		Actions: []Action{
			{At: 0, Pos: 0},
			{At: 500, Pos: 100},
			{At: 60500, Pos: 100},
			{At: 61000, Pos: 0},
		},
	}
	speed := script.CalculateSpeed()
	if speed != 200 {
		t.Fatalf("expected active moving speed 200 units/s, got %d", speed)
	}
}

func TestCalculateSpeed_EdgeCases(t *testing.T) {
	// N = 0
	scriptEmpty := Script{Actions: []Action{}}
	if s := scriptEmpty.CalculateSpeed(); s != 0 {
		t.Fatalf("expected speed 0, got %d", s)
	}

	// N = 1
	scriptSingle := Script{Actions: []Action{{At: 1000, Pos: 50}}}
	if s := scriptSingle.CalculateSpeed(); s != 0 {
		t.Fatalf("expected speed 0, got %d", s)
	}

	// Zero distance
	scriptZeroDist := Script{
		Actions: []Action{
			{At: 0, Pos: 50},
			{At: 1000, Pos: 50},
		},
	}
	if s := scriptZeroDist.CalculateSpeed(); s != 0 {
		t.Fatalf("expected speed 0, got %d", s)
	}

	// Token script
	tokenScript := Script{
		Actions: []Action{
			{At: 136740671 % 3, Pos: 0},
			{At: 1000, Pos: 1},
			{At: 2000, Pos: 2},
		},
	}
	if s := tokenScript.CalculateSpeed(); s != 0 {
		t.Fatalf("expected speed 0 for token, got %d", s)
	}
}

func TestCalculateSpeed_FromJSON(t *testing.T) {
	rawJSON := `{
		"version": "1.0",
		"actions": [
			{"at": 0, "pos": 0},
			{"at": 500, "pos": 100},
			{"at": 1000, "pos": 0}
		]
	}`

	var s Script
	if err := json.Unmarshal([]byte(rawJSON), &s); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	speed := s.CalculateSpeed()
	if speed != 200 {
		t.Fatalf("expected speed 200, got %d", speed)
	}
}

func TestCalculateAPM_Standard(t *testing.T) {
	script := Script{
		Actions: []Action{
			{At: 1500, Pos: 20},
			{At: 1850, Pos: 90},
			{At: 2200, Pos: 10},
		},
	}

	apm := script.CalculateAPM()
	if apm != 257 {
		t.Fatalf("expected APM 257, got %d", apm)
	}
}
