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
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// HumanLabel maps a dimension name (relevance, correctness, …) to a bucketed
// rating (low/med/high) for one case.
type HumanLabel map[string]string

// labelFile is the on-disk shape of a labeled-dataset YAML: case_id → per-
// dimension buckets.
//
//   - case_id: go-latest-version
//     scores:
//     relevance: high
//     correctness: high
type labelFile struct {
	CaseID string            `yaml:"case_id"`
	Scores map[string]string `yaml:"scores"`
}

// LoadLabels reads a labeled-dataset YAML and returns case_id → HumanLabel,
// validating that ids are unique and every bucket is low/med/high.
func LoadLabels(path string) (map[string]HumanLabel, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read labels: %w", err)
	}
	var entries []labelFile
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}
	out := make(map[string]HumanLabel, len(entries))
	for i, e := range entries {
		if e.CaseID == "" {
			return nil, fmt.Errorf("label entry %d: empty case_id", i)
		}
		if _, dup := out[e.CaseID]; dup {
			return nil, fmt.Errorf("duplicate case_id %q in labels", e.CaseID)
		}
		hl := make(HumanLabel, len(e.Scores))
		for dim, bucket := range e.Scores {
			if !validBucket(bucket) {
				return nil, fmt.Errorf("case %q dim %q: invalid bucket %q (want low/med/high)", e.CaseID, dim, bucket)
			}
			hl[dim] = bucket
		}
		out[e.CaseID] = hl
	}
	return out, nil
}

func validBucket(b string) bool {
	return b == "low" || b == "med" || b == "high"
}

// BucketScore maps a [0,1] score into low/med/high thirds, matching the human
// label granularity so judge scores can be compared against human buckets.
func BucketScore(v float64) string {
	switch {
	case v < 1.0/3.0:
		return "low"
	case v < 2.0/3.0:
		return "med"
	default:
		return "high"
	}
}

// AlignForDimension returns paired human and judge bucket slices for the given
// dimension, including only cases present (with that dimension) in BOTH maps.
// The pairing order is deterministic (sorted by case id) so kappa is stable.
func AlignForDimension(human, judge map[string]HumanLabel, dim string) (humanBuckets, judgeBuckets []string) {
	var ids []string
	for id, hl := range human {
		if _, ok := hl[dim]; ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		jl, ok := judge[id]
		if !ok {
			continue
		}
		jb, ok := jl[dim]
		if !ok {
			continue
		}
		humanBuckets = append(humanBuckets, human[id][dim])
		judgeBuckets = append(judgeBuckets, jb)
	}
	return humanBuckets, judgeBuckets
}

// CohenKappa computes Cohen's (unweighted) κ between two equal-length sequences
// of categorical ratings. κ = (po - pe) / (1 - pe), where po is observed
// agreement and pe is chance agreement from the marginal category frequencies.
//
// When 1 - pe is 0 (both raters used a single category for everything) κ is
// undefined; this returns 0, signaling "no variance to explain".
func CohenKappa(a, b []string) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("kappa: length mismatch %d vs %d", len(a), len(b))
	}
	n := len(a)
	if n == 0 {
		return 0, fmt.Errorf("kappa: empty input")
	}

	var agree int
	countA := map[string]int{}
	countB := map[string]int{}
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			agree++
		}
		countA[a[i]]++
		countB[b[i]]++
	}

	po := float64(agree) / float64(n)

	var pe float64
	for cat, ca := range countA {
		cb := countB[cat]
		pe += (float64(ca) / float64(n)) * (float64(cb) / float64(n))
	}

	denom := 1 - pe
	if denom == 0 {
		return 0, nil
	}
	return (po - pe) / denom, nil
}

// kappaDimensions are the judge dimensions validated against human labels.
var kappaDimensions = []string{"relevance", "correctness", "source_quality"}

// JudgeLabelsFromResults buckets the judge's per-dimension scores into
// low/med/high, keyed by case id, so they can be compared against human labels
// with CohenKappa. Errored cells are skipped; if model is non-empty, only that
// model's cells are included (human labels are tied to one model's answers).
func JudgeLabelsFromResults(results []Result, model string) map[string]HumanLabel {
	out := make(map[string]HumanLabel)
	for _, r := range results {
		if r.Err != "" || (model != "" && r.Model != model) {
			continue
		}
		out[r.CaseID] = HumanLabel{
			"relevance":      BucketScore(r.Scores.Relevance),
			"correctness":    BucketScore(r.Scores.Correctness),
			"source_quality": BucketScore(r.Scores.SourceQuality),
		}
	}
	return out
}

// KappaByDimension computes Cohen's κ per dimension between human labels and the
// judge's bucketed scores, returning κ and the paired-sample count per dimension.
// Dimensions with no overlapping cases are omitted from the κ map (count 0).
func KappaByDimension(human, judge map[string]HumanLabel) (kappa map[string]float64, counts map[string]int, err error) {
	kappa = make(map[string]float64)
	counts = make(map[string]int)
	for _, dim := range kappaDimensions {
		hb, jb := AlignForDimension(human, judge, dim)
		counts[dim] = len(hb)
		if len(hb) == 0 {
			continue
		}
		k, e := CohenKappa(hb, jb)
		if e != nil {
			return nil, nil, fmt.Errorf("kappa %s: %w", dim, e)
		}
		kappa[dim] = k
	}
	return kappa, counts, nil
}
