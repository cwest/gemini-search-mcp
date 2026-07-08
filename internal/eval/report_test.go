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
	"math"
	"strings"
	"testing"

	"github.com/cwest/gemini-search-mcp/internal/search"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCostUSD(t *testing.T) {
	// Use a model present in the pricing table.
	u := &search.Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000}
	p, ok := pricing["gemini-3.5-flash"]
	if !ok {
		t.Fatal("gemini-3.5-flash missing from pricing table")
	}
	want := p.InPerM + p.OutPerM
	if got := CostUSD("gemini-3.5-flash", u); !approx(got, want) {
		t.Errorf("CostUSD = %v, want %v", got, want)
	}
	if got := CostUSD("unknown-model", u); got != 0 {
		t.Errorf("CostUSD unknown = %v, want 0", got)
	}
	if got := CostUSD("gemini-3.5-flash", nil); got != 0 {
		t.Errorf("CostUSD nil usage = %v, want 0", got)
	}
}

func sampleResults() []Result {
	mk := func(model, cat string, rel, cor, sq float64, lat int64, cost float64, srcs int, errStr string) Result {
		return Result{
			CaseID:      cat + "-case",
			Category:    cat,
			Model:       model,
			Scores:      Scores{Relevance: rel, Correctness: cor, SourceQuality: sq},
			LatencyMS:   lat,
			CostUSD:     cost,
			SourceCount: srcs,
			Err:         errStr,
		}
	}
	return []Result{
		mk("flash", "factual", 1.0, 1.0, 1.0, 100, 0.001, 3, ""),
		mk("flash", "temporal", 0.5, 0.5, 0.5, 300, 0.003, 1, ""),
		mk("flash", "no-good-answer", 0.0, 0.0, 0.0, 200, 0.000, 0, "boom"),
		mk("pro", "factual", 0.8, 0.9, 0.7, 500, 0.01, 4, ""),
	}
}

func TestRenderAggregates(t *testing.T) {
	md, jsonBytes := Render(sampleResults())

	// JSON round-trips back to the same results.
	var got []Result
	if err := json.Unmarshal(jsonBytes, &got); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("json results len = %d, want 4", len(got))
	}

	// Per-model aggregate math for "flash":
	// successful cells = 2 (the errored one excluded from score/cost averages).
	// avg relevance = (1.0 + 0.5) / 2 = 0.75
	// p50 latency over all 3 cells (100,200,300) = 200
	// error rate = 1/3
	for _, want := range []string{
		"flash",
		"pro",
		"0.75", // flash avg relevance
		"200",  // flash p50 latency
		"factual",
		"temporal",
		"no-good-answer",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}

	// Table structure: a per-model table header and a per-category section.
	if !strings.Contains(md, "| Model |") {
		t.Errorf("markdown missing per-model table header\n%s", md)
	}
	if !strings.Contains(strings.ToLower(md), "category") {
		t.Errorf("markdown missing per-category breakdown\n%s", md)
	}
}

func TestAggregateMathDirect(t *testing.T) {
	aggs := aggregateByModel(sampleResults())
	var flash modelAgg
	found := false
	for _, a := range aggs {
		if a.Model == "flash" {
			flash = a
			found = true
		}
	}
	if !found {
		t.Fatal("flash aggregate missing")
	}
	if !approx(flash.AvgRelevance, 0.75) {
		t.Errorf("AvgRelevance = %v, want 0.75", flash.AvgRelevance)
	}
	if !approx(flash.AvgCorrectness, 0.75) {
		t.Errorf("AvgCorrectness = %v, want 0.75", flash.AvgCorrectness)
	}
	if flash.P50LatencyMS != 200 {
		t.Errorf("P50LatencyMS = %v, want 200", flash.P50LatencyMS)
	}
	if !approx(flash.ErrorRate, 1.0/3.0) {
		t.Errorf("ErrorRate = %v, want 1/3", flash.ErrorRate)
	}
	// $/1k queries = avg cost over successful cells * 1000.
	// successful flash costs = 0.001, 0.003 → avg 0.002 → *1000 = 2.0
	if !approx(flash.CostPer1k, 2.0) {
		t.Errorf("CostPer1k = %v, want 2.0", flash.CostPer1k)
	}
}
