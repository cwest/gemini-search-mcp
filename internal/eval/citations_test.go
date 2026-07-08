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
	"context"
	"math"
	"testing"
)

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCitationMathPrecision(t *testing.T) {
	// Precision = fraction of CITED sentences whose cited sources entail them.
	// 4 sentences: 2 cited+entailed, 1 cited+not-entailed, 1 uncited.
	// Uncited sentences are excluded from precision's denominator (ALCE).
	sents := []SentenceLabel{
		{Cited: true, Entailed: true},
		{Cited: true, Entailed: true},
		{Cited: true, Entailed: false},
		{Cited: false, Entailed: false},
	}
	stmts := []StatementLabel{}
	cr := citationScore(sents, stmts)
	// precision = 2 / 3
	if !approxEq(cr.Precision, 2.0/3.0) {
		t.Errorf("Precision = %v, want 2/3", cr.Precision)
	}
}

func TestCitationMathRecall(t *testing.T) {
	// Recall = fraction of source-supported statements that ARE cited.
	// 5 statements: 3 cited, 2 not.
	stmts := []StatementLabel{
		{Cited: true}, {Cited: true}, {Cited: true},
		{Cited: false}, {Cited: false},
	}
	cr := citationScore(nil, stmts)
	if !approxEq(cr.Recall, 3.0/5.0) {
		t.Errorf("Recall = %v, want 3/5", cr.Recall)
	}
}

func TestCitationMathEdgeCases(t *testing.T) {
	// No cited sentences → precision is vacuously 1.0 (nothing wrongly cited).
	cr := citationScore([]SentenceLabel{{Cited: false}}, nil)
	if cr.Precision != 1.0 {
		t.Errorf("Precision with no cited sentences = %v, want 1.0", cr.Precision)
	}
	// No source-supported statements → recall vacuously 1.0.
	cr = citationScore(nil, nil)
	if cr.Recall != 1.0 {
		t.Errorf("Recall with no statements = %v, want 1.0", cr.Recall)
	}
}

func TestCitationF1(t *testing.T) {
	cr := CitationResult{Precision: 0.5, Recall: 1.0}
	got := cr.F1()
	want := 2 * 0.5 * 1.0 / (0.5 + 1.0) // = 2/3
	if !approxEq(got, want) {
		t.Errorf("F1 = %v, want %v", got, want)
	}
	// Both zero → F1 zero (no divide-by-zero).
	if got := (CitationResult{}).F1(); got != 0 {
		t.Errorf("F1 of zero = %v, want 0", got)
	}
}

func TestParseSentenceLabels(t *testing.T) {
	raw := `{"sentences": [
		{"sentence": "A.", "cited": true, "entailed": true},
		{"sentence": "B.", "cited": false, "entailed": false}
	]}`
	got, err := parseSentenceLabels(raw)
	if err != nil {
		t.Fatalf("parseSentenceLabels: %v", err)
	}
	if len(got) != 2 || !got[0].Cited || !got[0].Entailed || got[1].Cited {
		t.Errorf("parsed = %+v", got)
	}
}

func TestParseStatementLabels(t *testing.T) {
	raw := `{"statements": [
		{"statement": "X.", "cited": true},
		{"statement": "Y.", "cited": false}
	]}`
	got, err := parseStatementLabels(raw)
	if err != nil {
		t.Fatalf("parseStatementLabels: %v", err)
	}
	if len(got) != 2 || !got[0].Cited || got[1].Cited {
		t.Errorf("parsed = %+v", got)
	}
}

func TestCitationsEndToEndWithStub(t *testing.T) {
	chat := &stubChat{responses: []string{
		// 1) precision pass: per-sentence cited/entailed labels
		`{"sentences": [
			{"sentence": "Go 1.26.4 is the latest stable release.", "cited": true, "entailed": true},
			{"sentence": "It is widely adopted.", "cited": true, "entailed": false}
		]}`,
		// 2) recall pass: source-supported statements and whether cited
		`{"statements": [
			{"statement": "Go 1.26.4 is the current stable release.", "cited": true},
			{"statement": "Go is maintained by Google.", "cited": false}
		]}`,
	}}

	cr, err := Citations(context.Background(), chat,
		"Go 1.26.4 is the latest stable release. It is widely adopted.",
		[]string{"Go 1.26.4 is the current stable release of Go, maintained by Google."})
	if err != nil {
		t.Fatalf("Citations: %v", err)
	}
	if !approxEq(cr.Precision, 0.5) {
		t.Errorf("Precision = %v, want 0.5", cr.Precision)
	}
	if !approxEq(cr.Recall, 0.5) {
		t.Errorf("Recall = %v, want 0.5", cr.Recall)
	}
	if len(chat.prompts) != 2 {
		t.Fatalf("expected 2 judge calls, got %d", len(chat.prompts))
	}
}
