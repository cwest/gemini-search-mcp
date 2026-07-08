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
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// DimAverages is the per-model average for each quality dimension, over
// successful cells. Faithfulness and citation fields are 0 in offline/Phase-1
// runs (they require the live fetch+judge path).
type DimAverages struct {
	Relevance         float64 `json:"relevance"`
	Correctness       float64 `json:"correctness"`
	SourceQuality     float64 `json:"source_quality"`
	Faithfulness      float64 `json:"faithfulness,omitempty"`
	CitationPrecision float64 `json:"citation_precision,omitempty"`
	CitationRecall    float64 `json:"citation_recall,omitempty"`
}

// Summary is the committed baseline shape: per-model dimension averages. It is
// the small, stable artifact a run is diffed against for regression gating.
type Summary struct {
	Models map[string]DimAverages `json:"models"`
}

// Regression is one dimension that dropped more than the threshold versus the
// baseline (or a model that disappeared entirely).
type Regression struct {
	Model     string  `json:"model"`
	Dimension string  `json:"dimension"`
	Baseline  float64 `json:"baseline"`
	Current   float64 `json:"current"`
	Delta     float64 `json:"delta"` // current - baseline (negative = regression)
	Missing   bool    `json:"missing,omitempty"`
}

// Summarize rolls results up into a baseline Summary, reusing the same per-model
// aggregation as the report so the two never drift.
func Summarize(results []Result) Summary {
	s := Summary{Models: map[string]DimAverages{}}
	for _, a := range aggregateByModel(results) {
		s.Models[a.Model] = DimAverages{
			Relevance:         a.AvgRelevance,
			Correctness:       a.AvgCorrectness,
			SourceQuality:     a.AvgSourceQual,
			Faithfulness:      a.AvgFaith,
			CitationPrecision: a.AvgCitePrec,
			CitationRecall:    a.AvgCiteRecall,
		}
	}
	return s
}

// Compare flags every dimension whose current value dropped more than threshold
// below the baseline, plus any baseline model missing from the current run.
// Results are sorted (model, dimension) for deterministic reporting.
func Compare(base, cur Summary, threshold float64) []Regression {
	var regs []Regression

	models := make([]string, 0, len(base.Models))
	for m := range base.Models {
		models = append(models, m)
	}
	sort.Strings(models)

	for _, m := range models {
		b := base.Models[m]
		c, ok := cur.Models[m]
		if !ok {
			regs = append(regs, Regression{Model: m, Dimension: "*", Missing: true})
			continue
		}
		for _, d := range dimsOf(b, c) {
			delta := d.cur - d.base
			if delta < -threshold {
				regs = append(regs, Regression{
					Model:     m,
					Dimension: d.name,
					Baseline:  d.base,
					Current:   d.cur,
					Delta:     delta,
				})
			}
		}
	}
	return regs
}

// dimPair pairs a dimension's baseline and current value for comparison.
type dimPair struct {
	name      string
	base, cur float64
}

func dimsOf(b, c DimAverages) []dimPair {
	return []dimPair{
		{"relevance", b.Relevance, c.Relevance},
		{"correctness", b.Correctness, c.Correctness},
		{"source_quality", b.SourceQuality, c.SourceQuality},
		{"faithfulness", b.Faithfulness, c.Faithfulness},
		{"citation_precision", b.CitationPrecision, c.CitationPrecision},
		{"citation_recall", b.CitationRecall, c.CitationRecall},
	}
}

// WriteSummary writes a baseline Summary as indented JSON.
func WriteSummary(path string, s Summary) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}
	return nil
}

// LoadSummary reads a committed baseline Summary.
func LoadSummary(path string) (Summary, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, fmt.Errorf("read baseline: %w", err)
	}
	var s Summary
	if err := json.Unmarshal(raw, &s); err != nil {
		return Summary{}, fmt.Errorf("parse baseline: %w", err)
	}
	if s.Models == nil {
		s.Models = map[string]DimAverages{}
	}
	return s, nil
}
