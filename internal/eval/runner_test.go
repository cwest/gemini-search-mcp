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
	"testing"

	"github.com/cwest/gemini-search-mcp/internal/search"
)

// stubSearcher returns a fixed answer + sources so runCell can be exercised
// without a live API call.
type stubSearcher struct {
	answer  string
	sources []search.Source
}

func (s stubSearcher) Search(_ context.Context, _ string) (*search.Result, error) {
	return &search.Result{Answer: s.answer, Sources: s.sources}, nil
}

// TestRunCellCapturesAnswerAndSources asserts that a completed cell records the
// model's answer text and cited sources so a labeled run is reproducible and can
// be reviewed against the exact material the judge saw.
//
// The judge is scripted through runCellWith's injected scorer so no network call
// happens; this test only asserts the captured answer/sources.
func TestRunCellCapturesAnswerAndSources(t *testing.T) {
	srcs := []search.Source{
		{Title: "Example", Domain: "example.com", URI: "https://example.com/a"},
	}
	client := stubSearcher{answer: "The answer is 42.", sources: srcs}
	c := Case{ID: "c1", Category: "factual", Query: "q"}

	scorer := scorerFunc(func(context.Context, Case, string, []search.Source) (Scores, error) {
		return Scores{Relevance: 1, Correctness: 1, SourceQuality: 1}, nil
	})

	res := runCellWith(context.Background(), client, "flash", c, scorer, Options{})

	if res.Answer != "The answer is 42." {
		t.Errorf("Answer = %q, want %q", res.Answer, "The answer is 42.")
	}
	if len(res.Sources) != 1 || res.Sources[0].Domain != "example.com" {
		t.Errorf("Sources = %+v, want one example.com source", res.Sources)
	}
}
