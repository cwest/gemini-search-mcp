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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwest/gemini-search-mcp/internal/search"
)

func loadFixture(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("judge_fixture_test.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return m
}

func TestParseVerdictFenced(t *testing.T) {
	fx := loadFixture(t)
	s, err := parseVerdict(fx["fenced"])
	if err != nil {
		t.Fatalf("parseVerdict: %v", err)
	}
	if s.Relevance != 1.0 || s.Correctness != 0.9 || s.SourceQuality != 0.95 {
		t.Errorf("scores = %+v", s)
	}
	if !strings.Contains(s.Reasoning, "Go 1.26.4") {
		t.Errorf("Reasoning = %q", s.Reasoning)
	}
}

func TestParseVerdictBare(t *testing.T) {
	fx := loadFixture(t)
	s, err := parseVerdict(fx["bare"])
	if err != nil {
		t.Fatalf("parseVerdict: %v", err)
	}
	if s.Relevance != 0.6 || s.Correctness != 0.7 || s.SourceQuality != 0.5 {
		t.Errorf("scores = %+v", s)
	}
}

func TestParseVerdictClamps(t *testing.T) {
	fx := loadFixture(t)
	s, err := parseVerdict(fx["out_of_range"])
	if err != nil {
		t.Fatalf("parseVerdict: %v", err)
	}
	if s.Relevance != 1.0 {
		t.Errorf("Relevance not clamped to 1.0: %v", s.Relevance)
	}
	if s.Correctness != 0.0 {
		t.Errorf("Correctness not clamped to 0.0: %v", s.Correctness)
	}
	if s.SourceQuality != 0.5 {
		t.Errorf("SourceQuality = %v", s.SourceQuality)
	}
}

func TestParseVerdictNoJSON(t *testing.T) {
	if _, err := parseVerdict("there is no json here at all"); err == nil {
		t.Fatal("expected error for input with no JSON object")
	}
}

func TestBuildPrompt(t *testing.T) {
	c := Case{
		ID:               "go-latest-version",
		Query:            "What is the latest stable version of Go?",
		Category:         "temporal",
		ExpectAssertions: []string{"names a Go 1.x version as the latest stable release"},
		ExpectDomains:    []string{"go.dev"},
	}
	answer := "Go 1.26.4 is the latest stable release."
	sources := []search.Source{
		{Title: "Go Downloads", Domain: "go.dev", URI: "https://go.dev/dl"},
	}
	p := buildPrompt(c, answer, sources)

	for _, want := range []string{
		c.Query,
		answer,
		"names a Go 1.x version as the latest stable release",
		"go.dev",
		"relevance",
		"correctness",
		"source_quality",
		"reasoning",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	// CoT-before-verdict and verbosity-penalty rubric language.
	if !strings.Contains(strings.ToLower(p), "verbos") {
		t.Errorf("prompt missing verbosity-penalty instruction")
	}
}
