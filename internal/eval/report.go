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
	"sort"
	"strings"

	"github.com/cwest/gemini-search-mcp/internal/search"
)

// Result is one case×model evaluation cell: the judge's scores plus operational
// metrics. Err is non-empty when the generation or judging of this cell failed.
//
// Faithfulness and Citations are only populated on live runs (they need fetched
// source text); they are nil otherwise.
type Result struct {
	CaseID       string              `json:"case_id"`
	Category     string              `json:"category"`
	Model        string              `json:"model"`
	Scores       Scores              `json:"scores"`
	Faithfulness *FaithfulnessResult `json:"faithfulness,omitempty"`
	Citations    *CitationResult     `json:"citations,omitempty"`
	LatencyMS    int64               `json:"latency_ms"`
	Usage        *search.Usage       `json:"usage,omitempty"`
	CostUSD      float64             `json:"cost_usd"`
	SourceCount  int                 `json:"source_count"`
	Err          string              `json:"err,omitempty"`
}

// modelAgg is the per-model rollup shown in the tradeoff table.
type modelAgg struct {
	Model          string
	N              int     // total cells
	Errors         int     // cells with Err set
	AvgRelevance   float64 // over successful cells
	AvgCorrectness float64 // over successful cells
	AvgSourceQual  float64 // over successful cells
	AvgFaith       float64 // over cells that have a faithfulness result
	NFaith         int     // count of those cells
	AvgCitePrec    float64 // over cells that have a citation result
	AvgCiteRecall  float64 // over cells that have a citation result
	NCite          int     // count of those cells
	P50LatencyMS   int64   // over all cells
	CostPer1k      float64 // avg cost over successful cells * 1000
	ErrorRate      float64 // errors / N
}

// categoryAgg is the per-category rollup (across all models).
type categoryAgg struct {
	Category       string
	N              int
	AvgRelevance   float64
	AvgCorrectness float64
	AvgSourceQual  float64
	ErrorRate      float64
}

// Render produces a human-readable markdown report and the raw JSON of the
// results. The markdown carries a per-model tradeoff table and a per-category
// breakdown.
func Render(results []Result) (string, []byte) {
	jsonBytes, _ := json.MarshalIndent(results, "", "  ")

	var b strings.Builder
	b.WriteString("# Eval Report\n\n")
	fmt.Fprintf(&b, "%d result cells across %d models.\n\n", len(results), countModels(results))

	b.WriteString("## Model tradeoff\n\n")
	b.WriteString("Averages over successful cells; p50 latency and error rate over all cells. ")
	b.WriteString("Cost is approximate (see pricing.go).\n\n")
	b.WriteString("| Model | Relevance | Correctness | Source quality | p50 latency (ms) | $/1k queries | Error rate |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, a := range aggregateByModel(results) {
		fmt.Fprintf(&b, "| %s | %.2f | %.2f | %.2f | %d | %.2f | %.0f%% |\n",
			a.Model, a.AvgRelevance, a.AvgCorrectness, a.AvgSourceQual,
			a.P50LatencyMS, a.CostPer1k, a.ErrorRate*100)
	}
	b.WriteString("\n")

	// Phase 2: faithfulness + citation precision/recall, only emitted when a
	// live run populated them.
	aggs := aggregateByModel(results)
	if anyPhase2(aggs) {
		b.WriteString("## Faithfulness & citations (Phase 2)\n\n")
		b.WriteString("Faithfulness = supported claims / total (claims checked against fetched source text). ")
		b.WriteString("Citation precision/recall are ALCE-style. Averages over the cells that have a live result.\n\n")
		b.WriteString("| Model | Faithfulness | Citation precision | Citation recall | Citation F1 |\n")
		b.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, a := range aggs {
			if a.NFaith == 0 && a.NCite == 0 {
				continue
			}
			f1 := CitationResult{Precision: a.AvgCitePrec, Recall: a.AvgCiteRecall}.F1()
			fmt.Fprintf(&b, "| %s | %.2f | %.2f | %.2f | %.2f |\n",
				a.Model, a.AvgFaith, a.AvgCitePrec, a.AvgCiteRecall, f1)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Category breakdown\n\n")
	b.WriteString("| Category | Cells | Relevance | Correctness | Source quality | Error rate |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, a := range aggregateByCategory(results) {
		fmt.Fprintf(&b, "| %s | %d | %.2f | %.2f | %.2f | %.0f%% |\n",
			a.Category, a.N, a.AvgRelevance, a.AvgCorrectness, a.AvgSourceQual, a.ErrorRate*100)
	}
	b.WriteString("\n")

	return b.String(), jsonBytes
}

// anyPhase2 reports whether any model has faithfulness or citation results.
func anyPhase2(aggs []modelAgg) bool {
	for _, a := range aggs {
		if a.NFaith > 0 || a.NCite > 0 {
			return true
		}
	}
	return false
}

func countModels(results []Result) int {
	seen := map[string]bool{}
	for _, r := range results {
		seen[r.Model] = true
	}
	return len(seen)
}

// aggregateByModel rolls up results per model, sorted by model name.
func aggregateByModel(results []Result) []modelAgg {
	type acc struct {
		n         int
		errors    int
		sumRel    float64
		sumCor    float64
		sumSQ     float64
		ok        int
		sumCost   float64
		latencies []int64
		sumFaith  float64
		nFaith    int
		sumPrec   float64
		sumRecall float64
		nCite     int
	}
	byModel := map[string]*acc{}
	var order []string
	for _, r := range results {
		a, ok := byModel[r.Model]
		if !ok {
			a = &acc{}
			byModel[r.Model] = a
			order = append(order, r.Model)
		}
		a.n++
		a.latencies = append(a.latencies, r.LatencyMS)
		if r.Err != "" {
			a.errors++
			continue
		}
		a.ok++
		a.sumRel += r.Scores.Relevance
		a.sumCor += r.Scores.Correctness
		a.sumSQ += r.Scores.SourceQuality
		a.sumCost += r.CostUSD
		if r.Faithfulness != nil {
			a.sumFaith += r.Faithfulness.Score
			a.nFaith++
		}
		if r.Citations != nil {
			a.sumPrec += r.Citations.Precision
			a.sumRecall += r.Citations.Recall
			a.nCite++
		}
	}
	sort.Strings(order)

	out := make([]modelAgg, 0, len(order))
	for _, m := range order {
		a := byModel[m]
		out = append(out, modelAgg{
			Model:          m,
			N:              a.n,
			Errors:         a.errors,
			AvgRelevance:   avg(a.sumRel, a.ok),
			AvgCorrectness: avg(a.sumCor, a.ok),
			AvgSourceQual:  avg(a.sumSQ, a.ok),
			AvgFaith:       avg(a.sumFaith, a.nFaith),
			NFaith:         a.nFaith,
			AvgCitePrec:    avg(a.sumPrec, a.nCite),
			AvgCiteRecall:  avg(a.sumRecall, a.nCite),
			NCite:          a.nCite,
			P50LatencyMS:   p50(a.latencies),
			CostPer1k:      avg(a.sumCost, a.ok) * 1000,
			ErrorRate:      avg(float64(a.errors), a.n),
		})
	}
	return out
}

// aggregateByCategory rolls up results per category, sorted by category name.
func aggregateByCategory(results []Result) []categoryAgg {
	type acc struct {
		n      int
		errors int
		ok     int
		sumRel float64
		sumCor float64
		sumSQ  float64
	}
	byCat := map[string]*acc{}
	var order []string
	for _, r := range results {
		a, ok := byCat[r.Category]
		if !ok {
			a = &acc{}
			byCat[r.Category] = a
			order = append(order, r.Category)
		}
		a.n++
		if r.Err != "" {
			a.errors++
			continue
		}
		a.ok++
		a.sumRel += r.Scores.Relevance
		a.sumCor += r.Scores.Correctness
		a.sumSQ += r.Scores.SourceQuality
	}
	sort.Strings(order)

	out := make([]categoryAgg, 0, len(order))
	for _, c := range order {
		a := byCat[c]
		out = append(out, categoryAgg{
			Category:       c,
			N:              a.n,
			AvgRelevance:   avg(a.sumRel, a.ok),
			AvgCorrectness: avg(a.sumCor, a.ok),
			AvgSourceQual:  avg(a.sumSQ, a.ok),
			ErrorRate:      avg(float64(a.errors), a.n),
		})
	}
	return out
}

func avg(sum float64, n int) float64 {
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// p50 returns the median of the latencies (lower-middle for even counts).
func p50(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := append([]int64(nil), xs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[(len(sorted)-1)/2]
}
