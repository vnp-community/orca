package domain

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Direction string

const (
	LowerIsBetter  Direction = "lower_is_better"
	HigherIsBetter Direction = "higher_is_better"
)

func (d Direction) Valid() bool { return d == LowerIsBetter || d == HigherIsBetter }

var ErrBaselineZero = errors.New("domain: baseline is zero")

// PerfBaselineMetric is one measured quantity with the improvement the plan promises.
type PerfBaselineMetric struct {
	Name                string    `json:"name"`
	Unit                string    `json:"unit"`
	Direction           Direction `json:"direction"`
	Baseline            float64   `json:"baseline"`
	TargetChangePercent float64   `json:"target_change_percent"`
}

type PerfBaseline struct {
	Metrics []PerfBaselineMetric `json:"metrics"`
	Method  string               `json:"method"`
}

type PerfAfterMetric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type PerfAfter struct {
	Metrics []PerfAfterMetric `json:"metrics"`
}

// ImprovementPercent is positive when value is better than baseline in the metric's direction.
func ImprovementPercent(baseline, value float64, dir Direction) (float64, error) {
	if baseline == 0 {
		return 0, ErrBaselineZero
	}
	if !dir.Valid() {
		return 0, fmt.Errorf("domain: unknown direction %q", dir)
	}
	change := (value - baseline) / baseline * 100
	if dir == LowerIsBetter {
		change = -change
	}
	return change, nil
}

func DecodePerfBaseline(raw json.RawMessage) (PerfBaseline, error) {
	var b PerfBaseline
	if err := json.Unmarshal(raw, &b); err != nil {
		return PerfBaseline{}, err
	}
	return b, nil
}

func DecodePerfAfter(raw json.RawMessage) (PerfAfter, error) {
	var a PerfAfter
	if err := json.Unmarshal(raw, &a); err != nil {
		return PerfAfter{}, err
	}
	return a, nil
}

// TestCounts is the payload of tests_before and tests_after.
type TestCounts struct {
	Total         int    `json:"total"`
	Passed        int    `json:"passed"`
	Failed        int    `json:"failed"`
	Command       string `json:"command"`
	TestsModified bool   `json:"tests_modified"`
}

func DecodeTestCounts(raw json.RawMessage) (TestCounts, error) {
	var c TestCounts
	if err := json.Unmarshal(raw, &c); err != nil {
		return TestCounts{}, err
	}
	return c, nil
}
