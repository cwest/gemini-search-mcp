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
	"strings"
	"time"

	"github.com/cwest/gemini-search-mcp/internal/search"
)

// Options controls optional Phase 2 scoring. When both Fetcher and Chat are set,
// each cell also fetches its cited source pages and scores faithfulness and
// citation precision/recall. They are nil for an offline/Phase-1 run.
//
// live path exercised by controller — the Phase 2 fields require real source
// fetching and live judge calls, so they are wired but never set in unit tests.
type Options struct {
	Fetcher Fetcher
	Chat    Completer
}

// Run evaluates every case against every model and scores each with the judge.
// It is sequential (the case×model set is small in Phase 1) and never aborts the
// whole run on a single failure: a generation or judging error is recorded in
// that cell's Result.Err and the run continues.
func Run(ctx context.Context, cases []Case, models []string, judge *Judge, opts Options) []Result {
	results := make([]Result, 0, len(cases)*len(models))
	for _, model := range models {
		client, err := search.New(ctx, model)
		if err != nil {
			// Whole model is unusable; record one error cell per case so the
			// report still accounts for it, then move on.
			for _, c := range cases {
				results = append(results, Result{
					CaseID:   c.ID,
					Category: c.Category,
					Model:    model,
					Err:      "init search client: " + err.Error(),
				})
			}
			continue
		}
		for _, c := range cases {
			results = append(results, runCell(ctx, client, model, c, judge, opts))
		}
	}
	return results
}

// runCell times one Search, computes its cost, and scores it with the judge.
func runCell(ctx context.Context, client search.Searcher, model string, c Case, judge *Judge, opts Options) Result {
	res := Result{CaseID: c.ID, Category: c.Category, Model: model}

	start := time.Now()
	sr, err := client.Search(ctx, c.Query)
	res.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		res.Err = "search: " + err.Error()
		return res
	}

	res.Usage = sr.Usage
	res.CostUSD = CostUSD(model, sr.Usage)
	res.SourceCount = len(sr.Sources)

	scores, err := judge.Score(ctx, c, sr.Answer, sr.Sources)
	if err != nil {
		res.Err = "judge: " + err.Error()
		return res
	}
	res.Scores = scores

	// Phase 2 (live only): fetch source pages, then score faithfulness and
	// citations. A fetch/score failure degrades gracefully — it leaves the
	// Phase 2 fields nil rather than failing the whole cell.
	if opts.Fetcher != nil && opts.Chat != nil {
		scorePhase2(ctx, &res, opts, sr)
	}
	return res
}

// scorePhase2 fetches the cell's cited sources and computes faithfulness and
// citation precision/recall. It is best-effort: any error leaves the relevant
// field unset.
//
// live path exercised by controller — depends on real HTTP fetches and judge
// calls, so it is not invoked from unit tests.
func scorePhase2(ctx context.Context, res *Result, opts Options, sr *search.Result) {
	texts := fetchSourceTexts(ctx, opts.Fetcher, sr.Sources)
	if len(texts) == 0 {
		return
	}
	joined := strings.Join(texts, "\n\n")

	if fr, err := Faithfulness(ctx, opts.Chat, sr.Answer, joined); err == nil {
		res.Faithfulness = &fr
	}
	if cr, err := Citations(ctx, opts.Chat, sr.Answer, texts); err == nil {
		res.Citations = &cr
	}
}

// fetchSourceTexts resolves and fetches each source, dropping any that fail.
func fetchSourceTexts(ctx context.Context, f Fetcher, sources []search.Source) []string {
	var texts []string
	for _, s := range sources {
		text, _, err := f.Fetch(ctx, s.URI)
		if err != nil || text == "" {
			continue
		}
		texts = append(texts, text)
	}
	return texts
}
