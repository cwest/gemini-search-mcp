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

import "testing"

func TestJudgeLabelsFromResults(t *testing.T) {
	results := []Result{
		{CaseID: "a", Model: "m1", Scores: Scores{Relevance: 0.9, Correctness: 0.5, SourceQuality: 0.2}},
		{CaseID: "b", Model: "m1", Scores: Scores{Relevance: 0.1}},
		{CaseID: "c", Model: "m1", Err: "boom"},                    // skipped (errored)
		{CaseID: "d", Model: "m2", Scores: Scores{Relevance: 0.9}}, // skipped (other model)
	}
	got := JudgeLabelsFromResults(results, "m1")
	if len(got) != 2 {
		t.Fatalf("got %d labeled cases, want 2 (errored + other-model skipped)", len(got))
	}
	if got["a"]["relevance"] != "high" || got["a"]["correctness"] != "med" || got["a"]["source_quality"] != "low" {
		t.Errorf("case a buckets = %+v", got["a"])
	}
	if got["b"]["relevance"] != "low" {
		t.Errorf("case b relevance = %q, want low", got["b"]["relevance"])
	}
}

func TestKappaByDimension(t *testing.T) {
	// Variance present (2 high, 1 low) and judge agrees perfectly on relevance.
	human := map[string]HumanLabel{
		"a": {"relevance": "high"},
		"b": {"relevance": "low"},
		"c": {"relevance": "high"},
	}
	judge := map[string]HumanLabel{
		"a": {"relevance": "high"},
		"b": {"relevance": "low"},
		"c": {"relevance": "high"},
	}
	kappa, counts, err := KappaByDimension(human, judge)
	if err != nil {
		t.Fatalf("KappaByDimension: %v", err)
	}
	if counts["relevance"] != 3 {
		t.Errorf("relevance count = %d, want 3", counts["relevance"])
	}
	if kappa["relevance"] != 1.0 {
		t.Errorf("relevance kappa = %v, want 1.0", kappa["relevance"])
	}
	if counts["correctness"] != 0 {
		t.Errorf("correctness count = %d, want 0 (no labels)", counts["correctness"])
	}
}
