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
	"testing"
)

// stubChat is a scripted Completer: each call returns the next queued response.
type stubChat struct {
	responses []string
	prompts   []string // captured for assertions
	i         int
}

func (s *stubChat) Complete(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if s.i >= len(s.responses) {
		return "", errNoMoreResponses
	}
	r := s.responses[s.i]
	s.i++
	return r, nil
}

var errNoMoreResponses = errStub("stubChat: no more responses queued")

type errStub string

func (e errStub) Error() string { return string(e) }

func TestParseClaims(t *testing.T) {
	raw := "```json\n{\"claims\": [\"Go 1.26.4 is the latest stable release.\", \"It was released in May 2026.\"]}\n```"
	got, err := parseClaims(raw)
	if err != nil {
		t.Fatalf("parseClaims: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d claims, want 2: %v", len(got), got)
	}
	if got[0] != "Go 1.26.4 is the latest stable release." {
		t.Errorf("claim[0] = %q", got[0])
	}
}

func TestParseClaimVerdicts(t *testing.T) {
	raw := `prose before
	{"verdicts": [
	  {"claim": "a", "label": "supported"},
	  {"claim": "b", "label": "PARTIAL"},
	  {"claim": "c", "label": "unsupported"}
	]}`
	got, err := parseClaimVerdicts(raw)
	if err != nil {
		t.Fatalf("parseClaimVerdicts: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d verdicts, want 3", len(got))
	}
	if got[0].Label != labelSupported {
		t.Errorf("verdict[0].Label = %q, want supported", got[0].Label)
	}
	if got[1].Label != labelPartial {
		t.Errorf("verdict[1].Label = %q (case-insensitive parse failed)", got[1].Label)
	}
	if got[2].Label != labelUnsupported {
		t.Errorf("verdict[2].Label = %q", got[2].Label)
	}
}

func TestParseClaimVerdictsUnknownLabel(t *testing.T) {
	raw := `{"verdicts": [{"claim": "a", "label": "maybe"}]}`
	if _, err := parseClaimVerdicts(raw); err == nil {
		t.Fatal("expected error for unknown label, got nil")
	}
}

func TestFaithfulnessScoreAggregation(t *testing.T) {
	// supported counts 1.0, partial 0.5, unsupported 0.0.
	// 2 supported + 1 partial + 1 unsupported over 4 → (2 + 0.5)/4 = 0.625.
	v := []ClaimVerdict{
		{Claim: "a", Label: labelSupported},
		{Claim: "b", Label: labelSupported},
		{Claim: "c", Label: labelPartial},
		{Claim: "d", Label: labelUnsupported},
	}
	got := faithfulnessScore(v)
	if got != 0.625 {
		t.Errorf("faithfulnessScore = %v, want 0.625", got)
	}
}

func TestFaithfulnessScoreNoClaims(t *testing.T) {
	if got := faithfulnessScore(nil); got != 0 {
		t.Errorf("faithfulnessScore(nil) = %v, want 0", got)
	}
}

func TestFaithfulnessEndToEndWithStub(t *testing.T) {
	chat := &stubChat{responses: []string{
		// 1) claim decomposition
		`{"claims": ["Go 1.26.4 is the latest stable release.", "It shipped with a new GC."]}`,
		// 2) claim classification vs source text
		`{"verdicts": [
			{"claim": "Go 1.26.4 is the latest stable release.", "label": "supported"},
			{"claim": "It shipped with a new GC.", "label": "unsupported"}
		]}`,
	}}

	fr, err := Faithfulness(context.Background(), chat,
		"Go 1.26.4 is the latest stable release. It shipped with a new GC.",
		"Go 1.26.4 is the current stable release of the Go programming language.")
	if err != nil {
		t.Fatalf("Faithfulness: %v", err)
	}
	if fr.Score != 0.5 {
		t.Errorf("Score = %v, want 0.5", fr.Score)
	}
	if len(fr.Verdicts) != 2 {
		t.Fatalf("got %d verdicts, want 2", len(fr.Verdicts))
	}
	// Both judge calls should have happened.
	if len(chat.prompts) != 2 {
		t.Fatalf("expected 2 judge calls, got %d", len(chat.prompts))
	}
	// The classification prompt must carry the source text.
	if !strings.Contains(chat.prompts[1], "current stable release of the Go") {
		t.Errorf("classification prompt missing source text:\n%s", chat.prompts[1])
	}
}

func TestFaithfulnessEmptyAnswer(t *testing.T) {
	chat := &stubChat{responses: []string{`{"claims": []}`}}
	fr, err := Faithfulness(context.Background(), chat, "", "some source")
	if err != nil {
		t.Fatalf("Faithfulness: %v", err)
	}
	if fr.Score != 0 || len(fr.Verdicts) != 0 {
		t.Errorf("empty answer should yield zero score and no verdicts, got %+v", fr)
	}
}
