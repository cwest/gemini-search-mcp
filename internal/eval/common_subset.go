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
	"fmt"
	"sort"
	"strings"
)

// This file enforces a single rule about "conditional" metrics — metrics that
// are only defined on a subset of cases (faithfulness and citation
// precision/recall/F1, which require a grounded answer with fetched sources).
//
// The rule: a conditional metric may only be compared HEAD-TO-HEAD across models
// on the COMMON grounded subset — the intersection of cases that EVERY compared
// model actually scored. Averaging a conditional metric over each model's own
// (variable-size) grounded subset and then ranking those averages produces a
// denominator artifact, not a real result. The report computes the intersection
// automatically and refuses to present a whole-table head-to-head when the
// per-model scored-n differ.

// commonSubset returns the sorted intersection of case IDs for which EVERY model
// present in results has a result satisfying present. A metric's cross-model
// comparison must be computed over exactly this set of cases.
func commonSubset(results []Result, present func(Result) bool) []string {
	// Per model, the set of case IDs where the metric is present.
	byModel := map[string]map[string]bool{}
	for _, r := range results {
		if _, ok := byModel[r.Model]; !ok {
			byModel[r.Model] = map[string]bool{}
		}
		if present(r) {
			byModel[r.Model][r.CaseID] = true
		}
	}
	if len(byModel) == 0 {
		return nil
	}

	// Intersect: a case is in the common subset iff every model has it.
	counts := map[string]int{}
	for _, cases := range byModel {
		for c := range cases {
			counts[c]++
		}
	}
	nModels := len(byModel)
	var out []string
	for c, n := range counts {
		if n == nModels {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// commonSubsetFaithfulness averages each model's faithfulness over the common
// grounded subset (the intersection of cases every model scored for
// faithfulness). It returns the per-model averages and n, the size of that
// intersection — the single denominator all models share. When the intersection
// is empty n is 0 and the map is empty.
func commonSubsetFaithfulness(results []Result) (map[string]float64, int) {
	subset := commonSubset(results, func(r Result) bool { return r.Faithfulness != nil })
	inSubset := toSet(subset)

	sum := map[string]float64{}
	cnt := map[string]int{}
	for _, r := range results {
		if r.Faithfulness == nil || !inSubset[r.CaseID] {
			continue
		}
		sum[r.Model] += r.Faithfulness.Score
		cnt[r.Model]++
	}
	out := map[string]float64{}
	for m, s := range sum {
		out[m] = avg(s, cnt[m])
	}
	return out, len(subset)
}

// commonSubsetCitations averages each model's citation precision and recall over
// the common citation subset (the intersection of cases every model produced a
// citation result for). Returns the per-model averaged CitationResult (P/R, from
// which F1 is derived) and n, the shared denominator.
func commonSubsetCitations(results []Result) (map[string]CitationResult, int) {
	subset := commonSubset(results, func(r Result) bool { return r.Citations != nil })
	inSubset := toSet(subset)

	sumP := map[string]float64{}
	sumR := map[string]float64{}
	cnt := map[string]int{}
	for _, r := range results {
		if r.Citations == nil || !inSubset[r.CaseID] {
			continue
		}
		sumP[r.Model] += r.Citations.Precision
		sumR[r.Model] += r.Citations.Recall
		cnt[r.Model]++
	}
	out := map[string]CitationResult{}
	for m := range cnt {
		out[m] = CitationResult{
			Precision: avg(sumP[m], cnt[m]),
			Recall:    avg(sumR[m], cnt[m]),
		}
	}
	return out, len(subset)
}

// hasVariableDenominator reports whether a conditional metric's per-model
// scored-n (via nOf) differs across models. When true, a whole-table
// head-to-head ranking on that metric is invalid and must be refused: the
// averages sit on different denominators. Fewer than two models is never
// variable (there is nothing to compare).
func hasVariableDenominator(aggs []modelAgg, nOf func(modelAgg) int) bool {
	var first int
	set := false
	for _, a := range aggs {
		n := nOf(a)
		if n == 0 {
			// Models with no scored cells for this metric don't participate in
			// the head-to-head at all, so they don't make the denominator vary.
			continue
		}
		if !set {
			first = n
			set = true
			continue
		}
		if n != first {
			return true
		}
	}
	return false
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// writePhase2 renders the conditional-metric (faithfulness + citation) section.
//
// It presents TWO structurally distinct tables:
//
//  1. The PRIMARY head-to-head, computed on the COMMON grounded subset — the
//     intersection of cases every model scored. This is the only apples-to-apples
//     cross-model comparison of these conditional metrics, so it leads. A single
//     shared denominator (n) is stated once for the whole table.
//
//  2. A per-model conditional table (faithfulness/citations WHEN GROUNDED) with
//     each model's own scored-n shown. This is explicitly NOT a head-to-head:
//     the per-model averages sit on each model's own denominator. When those
//     denominators differ, a guard note states plainly that the per-model
//     numbers must not be read as a ranking — the head-to-head lives in table 1.
func writePhase2(b *strings.Builder, results []Result, aggs []modelAgg) {
	b.WriteString("## Faithfulness & citations\n\n")
	b.WriteString("Faithfulness = supported claims / total (claims checked against fetched source text). ")
	b.WriteString("Citation precision/recall are ALCE-style. These metrics are only defined on a GROUNDED ")
	b.WriteString("answer (one that returned sources), so they are conditional metrics: comparing them ")
	b.WriteString("head-to-head is only valid on cases every model grounded.\n\n")

	// --- Table 1: primary head-to-head on the common grounded subset ---
	faith, nFaith := commonSubsetFaithfulness(results)
	cite, nCite := commonSubsetCitations(results)

	b.WriteString("### Head-to-head on the common grounded subset (apples-to-apples)\n\n")
	fmt.Fprintf(b, "Restricted to the cases every model grounded — faithfulness n=%d, citation n=%d. "+
		"This shared denominator is what makes the cross-model comparison valid.\n\n", nFaith, nCite)
	b.WriteString("| Model | Faithfulness | Citation precision | Citation recall | Citation F1 |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, a := range aggs {
		if a.NFaith == 0 && a.NCite == 0 {
			continue
		}
		cr := cite[a.Model]
		fmt.Fprintf(b, "| %s | %.3f | %.3f | %.3f | %.3f |\n",
			a.Model, faith[a.Model], cr.Precision, cr.Recall, cr.F1())
	}
	b.WriteString("\n")

	// --- Table 2: per-model conditional metrics, each on its own scored-n ---
	faithVaries := hasVariableDenominator(aggs, func(a modelAgg) int { return a.NFaith })
	citeVaries := hasVariableDenominator(aggs, func(a modelAgg) int { return a.NCite })

	b.WriteString("### Per-model conditional metrics (faithfulness / citations when grounded)\n\n")
	b.WriteString("Each row is averaged over that model's OWN grounded cases (scored-n shown). ")
	if faithVaries || citeVaries {
		b.WriteString("**The scored-n differ across models, so these columns are NOT a head-to-head: ")
		b.WriteString("their averages sit on different denominators. The cross-model ranking lives in ")
		b.WriteString("the common-grounded-subset table above, not here.**")
	} else {
		b.WriteString("Every model grounded the same number of cases here, so these equal the head-to-head above.")
	}
	b.WriteString("\n\n")
	b.WriteString("| Model | Faithfulness (scored-n) | Citation precision (scored-n) | Citation recall | Citation F1 |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, a := range aggs {
		if a.NFaith == 0 && a.NCite == 0 {
			continue
		}
		f1 := CitationResult{Precision: a.AvgCitePrec, Recall: a.AvgCiteRecall}.F1()
		fmt.Fprintf(b, "| %s | %.3f (n=%d) | %.3f (n=%d) | %.3f | %.3f |\n",
			a.Model, a.AvgFaith, a.NFaith, a.AvgCitePrec, a.NCite, a.AvgCiteRecall, f1)
	}
	b.WriteString("\n")
}
