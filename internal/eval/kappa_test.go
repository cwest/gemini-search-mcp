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
	"math"
	"path/filepath"
	"testing"
)

func TestCohenKappaPerfectAgreement(t *testing.T) {
	a := []string{"low", "med", "high", "high", "low"}
	b := []string{"low", "med", "high", "high", "low"}
	k, err := CohenKappa(a, b)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if math.Abs(k-1.0) > 1e-9 {
		t.Errorf("kappa = %v, want 1.0", k)
	}
}

func TestCohenKappaTotalDisagreementTwoCategories(t *testing.T) {
	// Each rater always picks the opposite of the other; expected agreement is
	// 0.5, observed 0, so kappa = -1.
	a := []string{"low", "low", "high", "high"}
	b := []string{"high", "high", "low", "low"}
	k, err := CohenKappa(a, b)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if math.Abs(k-(-1.0)) > 1e-9 {
		t.Errorf("kappa = %v, want -1.0", k)
	}
}

func TestCohenKappaChanceAgreementIsZero(t *testing.T) {
	// Both raters assign the same single category to everything: observed
	// agreement 1.0 but expected agreement is also 1.0, so kappa is 0 (defined
	// here as 0 when the denominator vanishes — no variance to explain).
	a := []string{"med", "med", "med"}
	b := []string{"med", "med", "med"}
	k, err := CohenKappa(a, b)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if k != 0 {
		t.Errorf("kappa = %v, want 0 (degenerate single-category case)", k)
	}
}

func TestCohenKappaKnownValue(t *testing.T) {
	// Classic 2x2 worked example (Cohen 1960 style):
	//        b=yes b=no
	// a=yes    20    5
	// a=no     10   15
	// po = (20+15)/50 = 0.70
	// pe = (25/50 * 30/50) + (25/50 * 20/50) = 0.30 + 0.20 = 0.50
	// kappa = (0.70 - 0.50) / (1 - 0.50) = 0.40
	var a, b []string
	add := func(av, bv string, n int) {
		for i := 0; i < n; i++ {
			a = append(a, av)
			b = append(b, bv)
		}
	}
	add("yes", "yes", 20)
	add("yes", "no", 5)
	add("no", "yes", 10)
	add("no", "no", 15)

	k, err := CohenKappa(a, b)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if math.Abs(k-0.40) > 1e-9 {
		t.Errorf("kappa = %v, want 0.40", k)
	}
}

func TestCohenKappaLengthMismatch(t *testing.T) {
	if _, err := CohenKappa([]string{"a"}, []string{"a", "b"}); err == nil {
		t.Fatal("expected length-mismatch error")
	}
}

func TestCohenKappaEmpty(t *testing.T) {
	if _, err := CohenKappa(nil, nil); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestBucketScore(t *testing.T) {
	cases := []struct {
		v    float64
		want string
	}{
		{0.0, "low"}, {0.32, "low"},
		{0.34, "med"}, {0.5, "med"}, {0.66, "med"},
		{0.67, "high"}, {1.0, "high"},
	}
	for _, c := range cases {
		if got := BucketScore(c.v); got != c.want {
			t.Errorf("BucketScore(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestLoadLabelsAndAlign(t *testing.T) {
	p := filepath.Join("..", "..", "evals", "labels", "example.yaml")
	labels, err := LoadLabels(p)
	if err != nil {
		t.Fatalf("LoadLabels: %v", err)
	}
	if len(labels) == 0 {
		t.Fatal("example labels file is empty")
	}
	// Every dimension bucket must be a valid bucket value.
	valid := map[string]bool{"low": true, "med": true, "high": true}
	for id, hl := range labels {
		for dim, bucket := range hl {
			if !valid[bucket] {
				t.Errorf("case %q dim %q has invalid bucket %q", id, dim, bucket)
			}
		}
	}
}

func TestAlignLabelsForDimension(t *testing.T) {
	human := map[string]HumanLabel{
		"c1": {"relevance": "high", "correctness": "low"},
		"c2": {"relevance": "med"},
		"c3": {"relevance": "low", "correctness": "high"},
	}
	judge := map[string]HumanLabel{
		"c1": {"relevance": "high", "correctness": "med"},
		"c2": {"relevance": "med"},
		"c3": {"relevance": "low", "correctness": "high"},
		"c4": {"relevance": "high"}, // no human label → excluded
	}
	h, j := AlignForDimension(human, judge, "relevance")
	if len(h) != 3 || len(j) != 3 {
		t.Fatalf("relevance aligned lengths = %d,%d, want 3,3", len(h), len(j))
	}
	k, err := CohenKappa(h, j)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if math.Abs(k-1.0) > 1e-9 {
		t.Errorf("relevance kappa = %v, want 1.0 (perfect on aligned set)", k)
	}

	// correctness present only on c1, c3.
	hc, jc := AlignForDimension(human, judge, "correctness")
	if len(hc) != 2 || len(jc) != 2 {
		t.Fatalf("correctness aligned lengths = %d,%d, want 2,2", len(hc), len(jc))
	}
}
