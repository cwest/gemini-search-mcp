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

func phase2Results() []Result {
	return []Result{
		{
			CaseID: "c1", Category: "factual", Model: "flash",
			Scores:       Scores{Relevance: 1, Correctness: 1, SourceQuality: 1},
			Faithfulness: &FaithfulnessResult{Score: 0.8},
			Citations:    &CitationResult{Precision: 0.9, Recall: 0.7},
		},
		{
			CaseID: "c2", Category: "factual", Model: "flash",
			Scores:       Scores{Relevance: 1, Correctness: 1, SourceQuality: 1},
			Faithfulness: &FaithfulnessResult{Score: 0.6},
			Citations:    &CitationResult{Precision: 0.7, Recall: 0.5},
		},
	}
}

func TestAggregatePhase2Dimensions(t *testing.T) {
	aggs := aggregateByModel(phase2Results())
	if len(aggs) != 1 {
		t.Fatalf("got %d model aggs, want 1", len(aggs))
	}
	a := aggs[0]
	if a.NFaith != 2 || a.NCite != 2 {
		t.Fatalf("NFaith=%d NCite=%d, want 2,2", a.NFaith, a.NCite)
	}
	if !approxEq(a.AvgFaith, 0.7) { // (0.8 + 0.6)/2
		t.Errorf("AvgFaith = %v, want 0.7", a.AvgFaith)
	}
	if !approxEq(a.AvgCitePrec, 0.8) { // (0.9 + 0.7)/2
		t.Errorf("AvgCitePrec = %v, want 0.8", a.AvgCitePrec)
	}
	if !approxEq(a.AvgCiteRecall, 0.6) { // (0.7 + 0.5)/2
		t.Errorf("AvgCiteRecall = %v, want 0.6", a.AvgCiteRecall)
	}
}

func TestRenderEmitsPhase2SectionWhenPresent(t *testing.T) {
	md, _ := Render(phase2Results())
	if !strings.Contains(md, "Faithfulness & citations (Phase 2)") {
		t.Errorf("phase 2 section missing:\n%s", md)
	}
	if !strings.Contains(md, "Citation F1") {
		t.Errorf("phase 2 table header missing:\n%s", md)
	}
}

func TestRenderOmitsPhase2SectionWhenAbsent(t *testing.T) {
	md, _ := Render(sampleResults()) // no faithfulness/citation results
	if strings.Contains(md, "Phase 2") {
		t.Errorf("phase 2 section should be absent:\n%s", md)
	}
}

func TestSummarizeIncludesPhase2(t *testing.T) {
	s := Summarize(phase2Results())
	flash := s.Models["flash"]
	if !approxEq(flash.Faithfulness, 0.7) {
		t.Errorf("Faithfulness = %v, want 0.7", flash.Faithfulness)
	}
	if !approxEq(flash.CitationPrecision, 0.8) {
		t.Errorf("CitationPrecision = %v, want 0.8", flash.CitationPrecision)
	}
	if !approxEq(flash.CitationRecall, 0.6) {
		t.Errorf("CitationRecall = %v, want 0.6", flash.CitationRecall)
	}
}

func TestCompareFlagsPhase2Regression(t *testing.T) {
	base := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.9, Faithfulness: 0.9, CitationPrecision: 0.9, CitationRecall: 0.9},
	}}
	cur := Summary{Models: map[string]DimAverages{
		"flash": {Relevance: 0.9, Faithfulness: 0.5, CitationPrecision: 0.9, CitationRecall: 0.9},
	}}
	regs := Compare(base, cur, 0.05)
	if len(regs) != 1 || regs[0].Dimension != "faithfulness" {
		t.Fatalf("expected faithfulness regression, got %+v", regs)
	}
}
