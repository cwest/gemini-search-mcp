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
	"math"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/vertex"

	"github.com/cwest/gemini-search-mcp/internal/search"
)

// defaultJudgeModel is the cross-family judge. A different model family from the
// system under test (Gemini) is the standard mitigation for self-preference bias.
const defaultJudgeModel = "claude-opus-4-8"

// judgeMaxTokens caps the judge response; the verdict is a small JSON object plus
// chain-of-thought reasoning.
const judgeMaxTokens = 2048

// Scores is the judge's per-dimension assessment of one answer. Each dimension is
// in [0,1]; Reasoning is the chain-of-thought that preceded the verdict.
type Scores struct {
	Reasoning     string  `json:"reasoning"`
	Relevance     float64 `json:"relevance"`
	Correctness   float64 `json:"correctness"`
	SourceQuality float64 `json:"source_quality"`
}

// Completer issues a single-prompt text completion and returns the model's text.
// The faithfulness and citation metrics depend on this interface (not the
// concrete Judge) so their logic is unit-tested with a scripted stub instead of
// a live API call.
type Completer interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// Judge scores Gemini answers with Claude running on Vertex AI.
type Judge struct {
	client anthropic.Client
	model  string
}

// Complete sends a single user prompt to the judge model and returns the
// concatenated text of the response. It satisfies Completer.
//
// live path exercised by controller — unit tests use a stub Completer.
func (j *Judge) Complete(ctx context.Context, prompt string) (string, error) {
	resp, err := j.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(j.model),
		MaxTokens: judgeMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("judge Messages.New: %w", err)
	}
	var raw strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			raw.WriteString(tb.Text)
		}
	}
	return raw.String(), nil
}

// NewJudge builds a Vertex-backed Claude judge. It reads GOOGLE_CLOUD_PROJECT
// (required) and GOOGLE_CLOUD_LOCATION (default "global"), and the model from
// EVAL_JUDGE_MODEL (default claude-opus-4-8). It makes no API call.
func NewJudge(ctx context.Context) (*Judge, error) {
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project == "" {
		return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT must be set for the Vertex judge")
	}
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if location == "" {
		location = "global"
	}
	model := os.Getenv("EVAL_JUDGE_MODEL")
	if model == "" {
		model = defaultJudgeModel
	}
	// Request the cloud-platform scope explicitly: WithGoogleAuth does not set a
	// default scope, and ADC token exchange fails with invalid_scope otherwise.
	client := anthropic.NewClient(vertex.WithGoogleAuth(ctx, location, project, "https://www.googleapis.com/auth/cloud-platform"))
	return &Judge{client: client, model: model}, nil
}

// Score runs the judge against one answer and returns the parsed verdict.
func (j *Judge) Score(ctx context.Context, c Case, answer string, sources []search.Source) (Scores, error) {
	raw, err := j.Complete(ctx, buildPrompt(c, answer, sources))
	if err != nil {
		return Scores{}, err
	}
	return parseVerdict(raw)
}

// buildPrompt renders the scoring rubric. It asks for chain-of-thought before the
// verdict, penalizes verbosity/filler, judges correctness against the case's
// expected assertions, and source quality against relevance + expected domains.
func buildPrompt(c Case, answer string, sources []search.Source) string {
	var b strings.Builder

	b.WriteString("You are a strict, impartial evaluator of web-search answers produced by a search system. ")
	b.WriteString("Judge the answer on three dimensions, each scored from 0.0 (worst) to 1.0 (best).\n\n")

	b.WriteString("Reason step by step BEFORE you commit to any score. ")
	b.WriteString("Be skeptical: do not reward confident-sounding but unsupported claims.\n\n")

	b.WriteString("Dimensions:\n")
	b.WriteString("- relevance: Does the answer directly address the query? ")
	b.WriteString("Penalize verbosity, padding, hedging, and filler that does not serve the query.\n")
	b.WriteString("- correctness: Does the answer satisfy the expected assertions below for this query's category? ")
	b.WriteString("For adversarial/no-good-answer cases, the correct behavior is to refuse or state that no answer exists rather than inventing one; ")
	b.WriteString("a hallucinated specific fact should score near 0.\n")
	b.WriteString("- source_quality: Are the cited sources relevant, reputable, and sufficient to support the answer? ")
	b.WriteString("Favor sources from the expected domains when listed, but a different reputable source is acceptable.\n\n")

	fmt.Fprintf(&b, "Query (category %q):\n%s\n\n", c.Category, c.Query)

	b.WriteString("Expected assertions a good answer should satisfy (semantic, not literal):\n")
	if len(c.ExpectAssertions) == 0 {
		b.WriteString("(none specified)\n")
	} else {
		for _, a := range c.ExpectAssertions {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	b.WriteString("\n")

	b.WriteString("Expected source domains (optional, for source_quality):\n")
	if len(c.ExpectDomains) == 0 {
		b.WriteString("(none specified)\n")
	} else {
		fmt.Fprintf(&b, "%s\n", strings.Join(c.ExpectDomains, ", "))
	}
	b.WriteString("\n")

	b.WriteString("Answer under evaluation:\n")
	b.WriteString(answer)
	b.WriteString("\n\n")

	b.WriteString("Cited sources:\n")
	if len(sources) == 0 {
		b.WriteString("(no sources returned)\n")
	} else {
		for i, s := range sources {
			fmt.Fprintf(&b, "%d. %s (%s) %s\n", i+1, s.Title, s.Domain, s.URI)
		}
	}
	b.WriteString("\n")

	b.WriteString("After reasoning, output a single strict JSON object and nothing else after it, with exactly these keys:\n")
	b.WriteString(`{"reasoning": string, "relevance": number, "correctness": number, "source_quality": number}`)
	b.WriteString("\nThe three numeric scores must be in [0,1].\n")

	return b.String()
}

// parseVerdict extracts the JSON verdict object from raw judge output, tolerating
// code fences and surrounding prose, and clamps each score to [0,1].
func parseVerdict(raw string) (Scores, error) {
	obj, err := extractJSONObject(raw)
	if err != nil {
		return Scores{}, err
	}
	var s Scores
	if err := json.Unmarshal([]byte(obj), &s); err != nil {
		return Scores{}, fmt.Errorf("unmarshal verdict: %w", err)
	}
	s.Relevance = clamp01(s.Relevance)
	s.Correctness = clamp01(s.Correctness)
	s.SourceQuality = clamp01(s.SourceQuality)
	return s, nil
}

// extractJSONObject returns the first balanced top-level {...} object in s. It
// ignores braces inside JSON string literals.
func extractJSONObject(s string) (string, error) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", fmt.Errorf("no JSON object found in judge output")
	}
	depth := 0
	inStr := false
	escaped := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inStr {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unbalanced JSON object in judge output")
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return math.Max(0, math.Min(1, v))
}
