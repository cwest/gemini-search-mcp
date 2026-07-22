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

import "testing"

// TestPricingCandidateModels pins the default-model-sweep candidate set to the
// official Vertex AI Standard-tier (Global) list prices, so the eval's cost/query
// figures reflect the real rate card rather than a stale approximation.
//
// Source: Vertex AI generative-ai pricing page, Gemini 3 family, Standard tier,
// Global region (USD per 1M tokens), captured 2026-07-22.
func TestPricingCandidateModels(t *testing.T) {
	cases := []struct {
		model   string
		inPerM  float64
		outPerM float64
	}{
		// Current default. Official Global price is $0.25 in / $1.50 out
		// (the prior $0.10/$0.40 entry was the Gemini 2.5 Flash-Lite rate).
		{"gemini-3.1-flash-lite", 0.25, 1.50},
		// Sweep candidates (GA 2026-07-21).
		{"gemini-3.6-flash", 1.50, 7.50},
		{"gemini-3.5-flash-lite", 0.30, 2.50},
	}
	for _, c := range cases {
		p, ok := pricing[c.model]
		if !ok {
			t.Errorf("pricing table missing %q", c.model)
			continue
		}
		if p.InPerM != c.inPerM {
			t.Errorf("%s InPerM = %v, want %v", c.model, p.InPerM, c.inPerM)
		}
		if p.OutPerM != c.outPerM {
			t.Errorf("%s OutPerM = %v, want %v", c.model, p.OutPerM, c.outPerM)
		}
	}
}
