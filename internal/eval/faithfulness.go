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

// Claim-support labels. A claim is checked against the fetched source text and
// gets exactly one of these.
const (
	labelSupported   = "supported"
	labelPartial     = "partial"
	labelUnsupported = "unsupported"
)

// ClaimVerdict is one decomposed claim and whether the source text backs it.
type ClaimVerdict struct {
	Claim string `json:"claim"`
	Label string `json:"label"`
}

// FaithfulnessResult is the per-answer groundedness score and its supporting
// per-claim verdicts.
type FaithfulnessResult struct {
	Score    float64        `json:"score"`
	Verdicts []ClaimVerdict `json:"verdicts"`
}

// Faithfulness measures whether an answer's claims are grounded in the fetched
// source text. It (1) decomposes the answer into atomic claims via the judge,
// then (2) classifies each claim against the source text as supported / partial
// / unsupported, and scores supported=1, partial=0.5, unsupported=0, averaged.
//
// Both steps go through the injected Completer, so this whole function is
// unit-tested with a scripted stub; the live judge path is exercised by the
// controller.
func Faithfulness(ctx context.Context, chat Completer, answer, sourceText string) (FaithfulnessResult, error) {
	rawClaims, err := chat.Complete(ctx, claimDecompositionPrompt(answer))
	if err != nil {
		return FaithfulnessResult{}, fmt.Errorf("decompose claims: %w", err)
	}
	claims, err := parseClaims(rawClaims)
	if err != nil {
		return FaithfulnessResult{}, fmt.Errorf("parse claims: %w", err)
	}
	if len(claims) == 0 {
		return FaithfulnessResult{}, nil
	}

	rawVerdicts, err := chat.Complete(ctx, claimClassificationPrompt(claims, sourceText))
	if err != nil {
		return FaithfulnessResult{}, fmt.Errorf("classify claims: %w", err)
	}
	verdicts, err := parseClaimVerdicts(rawVerdicts)
	if err != nil {
		return FaithfulnessResult{}, fmt.Errorf("parse verdicts: %w", err)
	}
	// The judge must return exactly one verdict per claim, in order; otherwise the
	// score (which divides by the verdict count) would be computed over the wrong
	// denominator. Fail loudly rather than silently inflate.
	if len(verdicts) != len(claims) {
		return FaithfulnessResult{}, fmt.Errorf("verdict count %d != claim count %d", len(verdicts), len(claims))
	}

	return FaithfulnessResult{
		Score:    faithfulnessScore(verdicts),
		Verdicts: verdicts,
	}, nil
}

// faithfulnessScore averages claim labels: supported=1, partial=0.5,
// unsupported=0. No verdicts → 0.
func faithfulnessScore(verdicts []ClaimVerdict) float64 {
	if len(verdicts) == 0 {
		return 0
	}
	var sum float64
	for _, v := range verdicts {
		switch v.Label {
		case labelSupported:
			sum += 1.0
		case labelPartial:
			sum += 0.5
		}
	}
	return sum / float64(len(verdicts))
}

func claimDecompositionPrompt(answer string) string {
	var b strings.Builder
	b.WriteString("Decompose the following answer into a list of atomic, self-contained factual claims. ")
	b.WriteString("Each claim must stand on its own (resolve pronouns and references), state exactly one fact, ")
	b.WriteString("and be verifiable against a source document. Ignore hedging, opinions, and meta-commentary.\n\n")
	b.WriteString("Answer:\n")
	b.WriteString(answer)
	b.WriteString("\n\n")
	b.WriteString("Output a single strict JSON object and nothing else, of the form:\n")
	b.WriteString(`{"claims": ["claim one", "claim two"]}`)
	b.WriteString("\nIf the answer contains no verifiable factual claims, return an empty list.\n")
	return b.String()
}

func claimClassificationPrompt(claims []string, sourceText string) string {
	var b strings.Builder
	b.WriteString("You are checking whether each claim is supported by the SOURCE TEXT below. ")
	b.WriteString("Judge ONLY against the source text, not your own knowledge.\n\n")
	b.WriteString("For each claim, assign exactly one label:\n")
	b.WriteString("- supported: the source text directly states or clearly entails the claim.\n")
	b.WriteString("- partial: the source text supports part of the claim but not all of it, or only weakly implies it.\n")
	b.WriteString("- unsupported: the source text does not support the claim, or contradicts it.\n\n")
	b.WriteString("SOURCE TEXT:\n")
	b.WriteString(sourceText)
	b.WriteString("\n\nCLAIMS:\n")
	for i, c := range claims {
		fmt.Fprintf(&b, "%d. %s\n", i+1, c)
	}
	b.WriteString("\nOutput a single strict JSON object and nothing else, of the form:\n")
	b.WriteString(`{"verdicts": [{"claim": "...", "label": "supported|partial|unsupported"}]}`)
	b.WriteString("\nReturn one verdict per claim, in order.\n")
	return b.String()
}

// parseClaims extracts the claims list from the decomposition response.
func parseClaims(raw string) ([]string, error) {
	obj, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var out struct {
		Claims []string `json:"claims"`
	}
	if err := json.Unmarshal([]byte(obj), &out); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	return out.Claims, nil
}

// parseClaimVerdicts extracts the per-claim verdicts, normalizing the label to
// lower-case and rejecting any label outside the known set.
func parseClaimVerdicts(raw string) ([]ClaimVerdict, error) {
	obj, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var out struct {
		Verdicts []ClaimVerdict `json:"verdicts"`
	}
	if err := json.Unmarshal([]byte(obj), &out); err != nil {
		return nil, fmt.Errorf("unmarshal verdicts: %w", err)
	}
	for i := range out.Verdicts {
		label := strings.ToLower(strings.TrimSpace(out.Verdicts[i].Label))
		switch label {
		case labelSupported, labelPartial, labelUnsupported:
			out.Verdicts[i].Label = label
		default:
			return nil, fmt.Errorf("verdict %d: unknown label %q", i, out.Verdicts[i].Label)
		}
	}
	return out.Verdicts, nil
}
