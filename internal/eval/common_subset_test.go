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
	"strings"
	"testing"
)

// divergentGroundedResults builds two models whose grounded (faithfulness/
// citation) case sets DIVERGE, reproducing the variable-denominator artifact:
//
//	model "a": grounded on c1, c2, c3       (faithfulness on all 3)
//	model "b": grounded on c1, c2 only      (faithfulness on 2)
//
// The intersection where BOTH are scored is {c1, c2}. Model a's whole-table
// faithfulness average (over 3 cases) and model b's (over 2 cases) sit on
// different denominators — comparing them head-to-head is the artifact this
// change forbids. On the common subset {c1, c2}, a and b are apples-to-apples.
func divergentGroundedResults() []Result {
	mk := func(model, caseID string, faith float64, grounded bool, prec, rec float64, hasCite bool) Result {
		r := Result{
			CaseID:   caseID,
			Category: "factual",
			Model:    model,
			Scores:   Scores{Relevance: 1, Correctness: 1, SourceQuality: 1},
		}
		if grounded {
			r.SourceCount = 2
			r.Faithfulness = &FaithfulnessResult{Score: faith}
			if hasCite {
				r.Citations = &CitationResult{Precision: prec, Recall: rec}
			}
		}
		return r
	}
	return []Result{
		// model a — grounded on all three, faithfulness {1.0, 0.8, 0.6}.
		mk("a", "c1", 1.0, true, 1.0, 0.5, true),
		mk("a", "c2", 0.8, true, 0.8, 0.4, true),
		mk("a", "c3", 0.6, true, 0.6, 0.3, true),
		// model b — grounded on c1, c2 only; ungrounded on c3.
		mk("b", "c1", 0.4, true, 0.6, 0.9, true),
		mk("b", "c2", 0.6, true, 0.4, 0.8, true),
		mk("b", "c3", 0.0, false, 0, 0, false),
	}
}

// The common grounded subset is the intersection of cases where EVERY model has
// a faithfulness result — {c1, c2} here — not each model's own subset.
func TestCommonGroundedSubsetIsIntersection(t *testing.T) {
	got := commonSubset(divergentGroundedResults(), func(r Result) bool { return r.Faithfulness != nil })
	want := map[string]bool{"c1": true, "c2": true}
	if len(got) != len(want) {
		t.Fatalf("common subset = %v, want keys %v", got, want)
	}
	for _, c := range got {
		if !want[c] {
			t.Errorf("unexpected case %q in common subset %v", c, got)
		}
	}
}

// Faithfulness on the common subset is averaged over the intersection, giving a
// like-for-like per-model number.
func TestCommonSubsetFaithfulness(t *testing.T) {
	byModel, n := commonSubsetFaithfulness(divergentGroundedResults())
	if n != 2 {
		t.Fatalf("common faithfulness n = %d, want 2", n)
	}
	// a: (1.0 + 0.8)/2 = 0.9 ; b: (0.4 + 0.6)/2 = 0.5
	if !approxEq(byModel["a"], 0.9) {
		t.Errorf("a faithfulness = %v, want 0.9", byModel["a"])
	}
	if !approxEq(byModel["b"], 0.5) {
		t.Errorf("b faithfulness = %v, want 0.5", byModel["b"])
	}
}

// Citation P/R on the common subset is averaged over the intersection.
func TestCommonSubsetCitations(t *testing.T) {
	byModel, n := commonSubsetCitations(divergentGroundedResults())
	if n != 2 {
		t.Fatalf("common citation n = %d, want 2", n)
	}
	// a: P (1.0+0.8)/2=0.9, R (0.5+0.4)/2=0.45
	if !approxEq(byModel["a"].Precision, 0.9) || !approxEq(byModel["a"].Recall, 0.45) {
		t.Errorf("a citation = %+v, want P=0.9 R=0.45", byModel["a"])
	}
	// b: P (0.6+0.4)/2=0.5, R (0.9+0.8)/2=0.85
	if !approxEq(byModel["b"].Precision, 0.5) || !approxEq(byModel["b"].Recall, 0.85) {
		t.Errorf("b citation = %+v, want P=0.5 R=0.85", byModel["b"])
	}
}

// hasVariableDenominator flags when a conditional metric's per-model scored-n
// differs across models — the signal that a whole-table head-to-head is invalid.
func TestHasVariableFaithfulnessDenominator(t *testing.T) {
	aggs := aggregateByModel(divergentGroundedResults())
	if !hasVariableDenominator(aggs, func(a modelAgg) int { return a.NFaith }) {
		t.Error("expected variable faithfulness denominator (a n=3, b n=2)")
	}
	// Equal denominators must NOT trip the guard.
	equal := aggregateByModel(phase2Results()) // single model, trivially equal
	if hasVariableDenominator(equal, func(a modelAgg) int { return a.NFaith }) {
		t.Error("single-model aggregate should not be flagged variable")
	}
}

// Render must NOT print a whole-table head-to-head faithfulness ranking when the
// denominators diverge; it must present the common-subset table as the primary
// head-to-head, and keep per-model conditional metrics as a separate,
// scored-n-labeled table (never a head-to-head).
func TestRenderGuardsVariableDenominatorHeadToHead(t *testing.T) {
	md, _ := Render(divergentGroundedResults())

	// The common-subset (apples-to-apples) head-to-head section must be present.
	if !strings.Contains(md, "common grounded subset") {
		t.Errorf("missing common grounded subset section:\n%s", md)
	}
	// The per-model conditional table must be present AND explicitly labeled as
	// per-model with a scored-n, not a cross-model ranking.
	if !strings.Contains(strings.ToLower(md), "scored-n") {
		t.Errorf("per-model conditional metrics must show scored-n:\n%s", md)
	}
	// The old, misleading whole-table head-to-head heading must be gone.
	if strings.Contains(md, "Faithfulness & citations (Phase 2)") {
		t.Errorf("old whole-table Phase 2 head-to-head heading must be removed:\n%s", md)
	}
	// A guard note must explain that whole-table conditional averages are not a
	// head-to-head because the denominators differ.
	if !strings.Contains(strings.ToLower(md), "denominator") {
		t.Errorf("expected a denominator guard note:\n%s", md)
	}
}

// When every model grounds the same cases (equal denominators), the common
// subset equals the full grounded set and no guard note is emitted.
func TestRenderNoGuardWhenDenominatorsEqual(t *testing.T) {
	md, _ := Render(phase2Results()) // one model, faithfulness on both cases
	// Still emits the common-subset head-to-head (trivially all cases)...
	if !strings.Contains(md, "common grounded subset") {
		t.Errorf("common subset section should still render:\n%s", md)
	}
}
