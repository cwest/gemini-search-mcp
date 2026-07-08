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
	"encoding/json"
	"fmt"
	"strings"
)

// SentenceLabel is one answer sentence, used for citation precision: whether it
// carries a citation and whether that cited source actually entails it.
type SentenceLabel struct {
	Sentence string `json:"sentence"`
	Cited    bool   `json:"cited"`
	Entailed bool   `json:"entailed"`
}

// StatementLabel is one source-supported statement, used for citation recall:
// whether the answer actually cites a source for it.
type StatementLabel struct {
	Statement string `json:"statement"`
	Cited     bool   `json:"cited"`
}

// CitationResult holds ALCE-style citation precision and recall.
type CitationResult struct {
	Precision       float64          `json:"precision"`
	Recall          float64          `json:"recall"`
	SentenceLabels  []SentenceLabel  `json:"sentence_labels,omitempty"`
	StatementLabels []StatementLabel `json:"statement_labels,omitempty"`
}

// F1 is the harmonic mean of precision and recall (0 when both are 0).
func (c CitationResult) F1() float64 {
	if c.Precision+c.Recall == 0 {
		return 0
	}
	return 2 * c.Precision * c.Recall / (c.Precision + c.Recall)
}

// Citations computes ALCE-style citation precision and recall via the judge.
//
// Precision pass: label each answer sentence as cited/entailed; precision is the
// fraction of CITED sentences whose cited sources entail them. Recall pass:
// enumerate the statements the sources support and whether the answer cites
// each; recall is the fraction of those statements that are cited.
//
// Both passes go through the injected Completer, so the logic is unit-tested with
// a stub; the live judge path is exercised by the controller.
func Citations(ctx context.Context, chat Completer, answer string, sourceTexts []string) (CitationResult, error) {
	joined := joinSources(sourceTexts)

	rawSents, err := chat.Complete(ctx, citationPrecisionPrompt(answer, joined))
	if err != nil {
		return CitationResult{}, fmt.Errorf("precision pass: %w", err)
	}
	sents, err := parseSentenceLabels(rawSents)
	if err != nil {
		return CitationResult{}, fmt.Errorf("parse sentence labels: %w", err)
	}

	rawStmts, err := chat.Complete(ctx, citationRecallPrompt(answer, joined))
	if err != nil {
		return CitationResult{}, fmt.Errorf("recall pass: %w", err)
	}
	stmts, err := parseStatementLabels(rawStmts)
	if err != nil {
		return CitationResult{}, fmt.Errorf("parse statement labels: %w", err)
	}

	cr := citationScore(sents, stmts)
	cr.SentenceLabels = sents
	cr.StatementLabels = stmts
	return cr, nil
}

// citationScore computes precision from per-sentence labels and recall from
// per-statement labels. Empty denominators are vacuously 1.0 (nothing wrong).
func citationScore(sents []SentenceLabel, stmts []StatementLabel) CitationResult {
	var cited, entailedCited int
	for _, s := range sents {
		if s.Cited {
			cited++
			if s.Entailed {
				entailedCited++
			}
		}
	}
	precision := 1.0
	if cited > 0 {
		precision = float64(entailedCited) / float64(cited)
	}

	var citedStmts int
	for _, st := range stmts {
		if st.Cited {
			citedStmts++
		}
	}
	recall := 1.0
	if len(stmts) > 0 {
		recall = float64(citedStmts) / float64(len(stmts))
	}

	return CitationResult{Precision: precision, Recall: recall}
}

func joinSources(sourceTexts []string) string {
	var b strings.Builder
	for i, s := range sourceTexts {
		fmt.Fprintf(&b, "[%d] %s\n\n", i+1, s)
	}
	return b.String()
}

func citationPrecisionPrompt(answer, sources string) string {
	var b strings.Builder
	b.WriteString("Split the ANSWER into sentences. For each sentence, decide:\n")
	b.WriteString("- cited: does the sentence make a factual claim that is attributed to one of the SOURCES ")
	b.WriteString("(i.e., the sentence draws on the source material rather than being filler or a generality)?\n")
	b.WriteString("- entailed: is the sentence actually supported (entailed) by the SOURCES it relies on?\n\n")
	b.WriteString("SOURCES:\n")
	b.WriteString(sources)
	b.WriteString("\nANSWER:\n")
	b.WriteString(answer)
	b.WriteString("\n\nOutput a single strict JSON object and nothing else, of the form:\n")
	b.WriteString(`{"sentences": [{"sentence": "...", "cited": true, "entailed": true}]}`)
	b.WriteString("\n")
	return b.String()
}

func citationRecallPrompt(answer, sources string) string {
	var b strings.Builder
	b.WriteString("Enumerate the distinct factual statements that the SOURCES support and that are relevant to the ANSWER. ")
	b.WriteString("For each such statement, decide whether the ANSWER actually states/cites it.\n\n")
	b.WriteString("- cited: does the ANSWER include this source-supported statement?\n\n")
	b.WriteString("SOURCES:\n")
	b.WriteString(sources)
	b.WriteString("\nANSWER:\n")
	b.WriteString(answer)
	b.WriteString("\n\nOutput a single strict JSON object and nothing else, of the form:\n")
	b.WriteString(`{"statements": [{"statement": "...", "cited": true}]}`)
	b.WriteString("\n")
	return b.String()
}

func parseSentenceLabels(raw string) ([]SentenceLabel, error) {
	obj, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var out struct {
		Sentences []SentenceLabel `json:"sentences"`
	}
	if err := json.Unmarshal([]byte(obj), &out); err != nil {
		return nil, fmt.Errorf("unmarshal sentence labels: %w", err)
	}
	return out.Sentences, nil
}

func parseStatementLabels(raw string) ([]StatementLabel, error) {
	obj, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var out struct {
		Statements []StatementLabel `json:"statements"`
	}
	if err := json.Unmarshal([]byte(obj), &out); err != nil {
		return nil, fmt.Errorf("unmarshal statement labels: %w", err)
	}
	return out.Statements, nil
}
