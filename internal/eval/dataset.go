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

// Package eval is the offline-and-live evaluation harness for the search tool:
// a golden dataset, a cross-family LLM judge, and report aggregation.
package eval

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Case is one golden query plus the expectations a good answer should meet.
// Assertions and domains are judge-checked (semantic), not string-matched.
type Case struct {
	ID               string   `yaml:"id"`
	Query            string   `yaml:"query"`
	Category         string   `yaml:"category"`
	ExpectAssertions []string `yaml:"expect_assertions"`
	ExpectDomains    []string `yaml:"expect_domains"`
	Notes            string   `yaml:"notes"`
}

// Load reads the YAML dataset at path, validating that every case has a
// non-empty ID and Query and that IDs are unique.
func Load(path string) ([]Case, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset: %w", err)
	}
	var cases []Case
	if err := yaml.Unmarshal(raw, &cases); err != nil {
		return nil, fmt.Errorf("parse dataset: %w", err)
	}
	seen := make(map[string]bool, len(cases))
	for i, c := range cases {
		if c.ID == "" {
			return nil, fmt.Errorf("case %d: empty id", i)
		}
		if c.Query == "" {
			return nil, fmt.Errorf("case %q: empty query", c.ID)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("duplicate case id %q", c.ID)
		}
		seen[c.ID] = true
	}
	return cases, nil
}
