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

// Command eval runs the golden query set through one or more Gemini models,
// scores each answer with the Claude-on-Vertex judge, and writes a markdown +
// JSON report with a model-tradeoff table.
//
// It makes paid live API calls (Gemini generation + Claude judging) and is gated
// behind RUN_EVALS=1 so it never runs accidentally or in PR CI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cwest/gemini-search-mcp/internal/eval"
)

func main() {
	dataset := flag.String("dataset", "evals/dataset/cases.yaml", "path to the golden dataset YAML")
	modelsCSV := flag.String("models", "gemini-3.1-pro-preview,gemini-3.5-flash,gemini-3.1-flash-lite",
		"comma-separated Gemini model ids to evaluate")
	outDir := flag.String("out", "./eval-results", "directory for the timestamped report files")
	phase2 := flag.Bool("phase2", false,
		"also score faithfulness + citation precision/recall (fetches source pages; more judge calls)")
	baseline := flag.String("baseline", "", "path to a committed baseline summary JSON to gate against")
	threshold := flag.Float64("threshold", 0.05, "max allowed per-dimension drop vs baseline before failing")
	writeBaseline := flag.String("write-baseline", "", "write this run's summary to this path (e.g. to refresh the baseline)")
	kappaLabels := flag.String("kappa", "", "compute Cohen's κ (judge vs human labels at this YAML) and exit; offline, requires --results")
	kappaResults := flag.String("results", "", "path to an eval-results JSON file (used with --kappa)")
	kappaModel := flag.String("kappa-model", "", "with --kappa, restrict to this model's cells (default: all)")
	flag.Parse()

	// Cohen's κ validation is offline (reads a prior results JSON + human labels),
	// so it runs before the live-eval gate.
	if *kappaLabels != "" {
		if *kappaResults == "" {
			log.Fatal("--kappa requires --results <eval-results JSON>")
		}
		if err := runKappa(*kappaLabels, *kappaResults, *kappaModel); err != nil {
			log.Fatalf("kappa: %v", err)
		}
		return
	}

	if os.Getenv("RUN_EVALS") != "1" {
		fmt.Fprintln(os.Stderr,
			"eval makes paid live API calls (Gemini + Claude on Vertex). "+
				"Set RUN_EVALS=1 and the Vertex env (GOOGLE_CLOUD_PROJECT, GOOGLE_CLOUD_LOCATION, "+
				"credentials) to run it.")
		os.Exit(2)
	}

	models := splitModels(*modelsCSV)
	if len(models) == 0 {
		log.Fatal("no models specified")
	}

	cases, err := eval.Load(*dataset)
	if err != nil {
		log.Fatalf("load dataset: %v", err)
	}
	log.Printf("loaded %d cases; models: %s", len(cases), strings.Join(models, ", "))

	ctx := context.Background()
	judge, err := eval.NewJudge(ctx)
	if err != nil {
		log.Fatalf("init judge: %v", err)
	}

	var opts eval.Options
	if *phase2 {
		// Phase 2 reuses the same Claude-on-Vertex judge as a Completer and an
		// HTTP fetcher to resolve the Vertex grounding redirect URIs.
		opts.Fetcher = eval.NewHTTPFetcher()
		opts.Chat = judge
		log.Print("phase 2 enabled: faithfulness + citation scoring (source fetch + extra judge calls)")
	}

	results := eval.Run(ctx, cases, models, judge, opts)
	md, jsonBytes := eval.Render(results)

	summary := eval.Summarize(results)
	if *writeBaseline != "" {
		if err := eval.WriteSummary(*writeBaseline, summary); err != nil {
			log.Fatalf("write baseline: %v", err)
		}
		log.Printf("wrote baseline summary to %s", *writeBaseline)
	}

	stamp := time.Now().UTC().Format(time.RFC3339)
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("create out dir: %v", err)
	}
	mdPath := filepath.Join(*outDir, stamp+".md")
	jsonPath := filepath.Join(*outDir, stamp+".json")
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		log.Fatalf("write markdown: %v", err)
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		log.Fatalf("write json: %v", err)
	}
	log.Printf("wrote %s and %s", mdPath, jsonPath)

	fmt.Print(md)

	// Regression gate: compare against a committed baseline and fail the run if
	// any aggregate dimension dropped more than the threshold.
	if *baseline != "" {
		base, err := eval.LoadSummary(*baseline)
		if err != nil {
			log.Fatalf("load baseline: %v", err)
		}
		regs := eval.Compare(base, summary, *threshold)
		if len(regs) > 0 {
			for _, r := range regs {
				if r.Missing {
					log.Printf("REGRESSION: model %q missing from this run (was in baseline)", r.Model)
					continue
				}
				log.Printf("REGRESSION: %s %s %.3f -> %.3f (delta %.3f, threshold %.3f)",
					r.Model, r.Dimension, r.Baseline, r.Current, r.Delta, *threshold)
			}
			log.Fatalf("%d regression(s) exceeded threshold %.3f", len(regs), *threshold)
		}
		log.Printf("no regressions vs baseline %s (threshold %.3f)", *baseline, *threshold)
	}
}

// runKappa computes Cohen's κ between the judge's bucketed scores (from a prior
// eval-results JSON) and human labels, per dimension. Offline; no API calls.
func runKappa(labelsPath, resultsPath, model string) error {
	raw, err := os.ReadFile(resultsPath)
	if err != nil {
		return fmt.Errorf("read results: %w", err)
	}
	var results []eval.Result
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("parse results JSON: %w", err)
	}
	human, err := eval.LoadLabels(labelsPath)
	if err != nil {
		return err
	}
	judge := eval.JudgeLabelsFromResults(results, model)
	kappa, counts, err := eval.KappaByDimension(human, judge)
	if err != nil {
		return err
	}
	fmt.Println("Cohen's κ — judge vs human labels, by dimension:")
	fmt.Printf("%-16s %8s %5s\n", "dimension", "kappa", "n")
	for _, dim := range []string{"relevance", "correctness", "source_quality"} {
		if counts[dim] == 0 {
			fmt.Printf("%-16s %8s %5d\n", dim, "-", 0)
			continue
		}
		fmt.Printf("%-16s %8.3f %5d\n", dim, kappa[dim], counts[dim])
	}
	fmt.Println("\nκ > 0.6 indicates acceptable judge-human agreement. Replace the")
	fmt.Println("placeholder rows in the labels file with real human ratings first.")
	return nil
}

func splitModels(csv string) []string {
	var out []string
	for _, m := range strings.Split(csv, ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}
