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
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "cases.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return p
}

func TestLoadParses(t *testing.T) {
	p := writeTemp(t, `
- id: go-latest-version
  query: "What is the latest stable version of Go?"
  category: temporal
  expect_assertions:
    - "names a Go 1.x version as the latest stable release"
  expect_domains: ["go.dev"]
  notes: "freshness"
- id: nonexistent-fact
  query: "What is the population of the Martian city of Bradbury Heights?"
  category: no-good-answer
  expect_assertions:
    - "states there is no such city"
`)
	cases, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("len = %d, want 2", len(cases))
	}
	c := cases[0]
	if c.ID != "go-latest-version" || c.Category != "temporal" {
		t.Errorf("case0 = %+v", c)
	}
	if len(c.ExpectAssertions) != 1 || c.ExpectAssertions[0] != "names a Go 1.x version as the latest stable release" {
		t.Errorf("ExpectAssertions = %v", c.ExpectAssertions)
	}
	if len(c.ExpectDomains) != 1 || c.ExpectDomains[0] != "go.dev" {
		t.Errorf("ExpectDomains = %v", c.ExpectDomains)
	}
	if c.Notes != "freshness" {
		t.Errorf("Notes = %q", c.Notes)
	}
}

func TestLoadDuplicateID(t *testing.T) {
	p := writeTemp(t, `
- id: dup
  query: "q1"
  category: factual
- id: dup
  query: "q2"
  category: factual
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected duplicate-ID error, got nil")
	}
}

func TestLoadMissingFields(t *testing.T) {
	p := writeTemp(t, `
- id: ""
  query: "q"
  category: factual
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}

	p2 := writeTemp(t, `
- id: x
  query: ""
  category: factual
`)
	if _, err := Load(p2); err == nil {
		t.Fatal("expected error for empty Query, got nil")
	}
}

func TestLoadGoldenDataset(t *testing.T) {
	p := filepath.Join("..", "..", "evals", "dataset", "cases.yaml")
	cases, err := Load(p)
	if err != nil {
		t.Fatalf("Load golden: %v", err)
	}
	if len(cases) < 12 {
		t.Errorf("golden has %d cases, want >= 12", len(cases))
	}
	cats := map[string]bool{}
	for _, c := range cases {
		if c.ID == "" || c.Query == "" || c.Category == "" {
			t.Errorf("case missing required field: %+v", c)
		}
		cats[c.Category] = true
	}
	for _, want := range []string{"factual", "temporal", "how-to", "multi-hop", "ambiguous", "no-good-answer"} {
		if !cats[want] {
			t.Errorf("golden dataset missing category %q", want)
		}
	}
}
