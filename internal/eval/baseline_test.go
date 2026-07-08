// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSummarizeAggregatesPerModel(t *testing.T) {
	got := Summarize(sampleResults())
	flash, ok := got.Models["flash"]
	if !ok {
		t.Fatal("summary missing flash")
	}
	// From sampleResults: flash successful relevance avg = (1.0 + 0.5)/2 = 0.75.
	if !approxEq(flash.Relevance, 0.75) {
		t.Errorf("flash.Relevance = %v, want 0.75", flash.Relevance)
	}
	if !approxEq(flash.Correctness, 0.75) {
		t.Errorf("flash.Correctness = %v, want 0.75", flash.Correctness)
	}
}

func TestCompareNoRegression(t *testing.T) {
	base := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.80, Correctness: 0.80, SourceQuality: 0.50},
	}}
	cur := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.82, Correctness: 0.79, SourceQuality: 0.55},
	}}
	// Threshold 0.05: correctness dropped only 0.01, within tolerance.
	regs := Compare(base, cur, 0.05)
	if len(regs) != 0 {
		t.Errorf("expected no regressions, got %+v", regs)
	}
}

func TestCompareFlagsDropBeyondThreshold(t *testing.T) {
	base := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.90, Correctness: 0.90, SourceQuality: 0.60},
	}}
	cur := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.90, Correctness: 0.70, SourceQuality: 0.60},
	}}
	regs := Compare(base, cur, 0.05)
	if len(regs) != 1 {
		t.Fatalf("expected 1 regression, got %d: %+v", len(regs), regs)
	}
	r := regs[0]
	if r.Model != "flash" || r.Dimension != "correctness" {
		t.Errorf("regression = %+v, want flash/correctness", r)
	}
	if !approxEq(r.Delta, -0.20) {
		t.Errorf("Delta = %v, want -0.20", r.Delta)
	}
}

func TestCompareImprovementIsNotRegression(t *testing.T) {
	base := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.50},
	}}
	cur := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.90},
	}}
	if regs := Compare(base, cur, 0.05); len(regs) != 0 {
		t.Errorf("improvement flagged as regression: %+v", regs)
	}
}

func TestCompareMissingModelInCurrent(t *testing.T) {
	// A model present in the baseline but absent from the current run is a
	// regression (we lost coverage / the model errored out entirely).
	base := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.90},
		"pro":   {Relevance: 0.80},
	}}
	cur := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.90},
	}}
	regs := Compare(base, cur, 0.05)
	if len(regs) != 1 {
		t.Fatalf("expected 1 regression for missing model, got %+v", regs)
	}
	if regs[0].Model != "pro" || !regs[0].Missing {
		t.Errorf("regression = %+v, want pro/Missing", regs[0])
	}
}

func TestSummaryRoundTrip(t *testing.T) {
	s := Summarize(sampleResults())
	dir := t.TempDir()
	p := filepath.Join(dir, "baseline.json")
	if err := WriteSummary(p, s); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}
	loaded, err := LoadSummary(p)
	if err != nil {
		t.Fatalf("LoadSummary: %v", err)
	}
	if len(loaded.Models) != len(s.Models) {
		t.Fatalf("round-trip model count = %d, want %d", len(loaded.Models), len(s.Models))
	}
	if !approxEq(loaded.Models["flash"].Relevance, s.Models["flash"].Relevance) {
		t.Errorf("round-trip flash relevance = %v, want %v",
			loaded.Models["flash"].Relevance, s.Models["flash"].Relevance)
	}
}

func TestLoadSummaryMissingFile(t *testing.T) {
	if _, err := LoadSummary(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error loading missing baseline")
	}
	// Sanity: an empty (zero) summary writes and reads back.
	dir := t.TempDir()
	p := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(p, []byte(`{"models":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSummary(p); err != nil {
		t.Fatalf("LoadSummary empty: %v", err)
	}
}
