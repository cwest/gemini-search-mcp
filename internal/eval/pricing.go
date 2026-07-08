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

import "github.com/cwest/gemini-search-mcp/internal/search"

// modelPrice is the per-million-token price of a model, in USD.
type modelPrice struct {
	InPerM  float64 // USD per 1M input tokens
	OutPerM float64 // USD per 1M output tokens
}

// pricing holds approximate Gemini list prices, as of 2026-06. These are
// hardcoded and may drift from the live rate card; update as needed. Unknown
// models are treated as $0 by CostUSD (see the caveat there).
var pricing = map[string]modelPrice{
	"gemini-3.1-pro-preview": {InPerM: 1.25, OutPerM: 10.00},
	"gemini-3.5-flash":       {InPerM: 0.30, OutPerM: 2.50},
	"gemini-3.1-flash-lite":  {InPerM: 0.10, OutPerM: 0.40},
}

// CostUSD returns the dollar cost of one grounded call given its token usage.
// Thought tokens are billed at the output rate. An unknown model or nil usage
// yields 0 — callers reporting cost should note that 0 may mean "unpriced".
func CostUSD(model string, u *search.Usage) float64 {
	if u == nil {
		return 0
	}
	p, ok := pricing[model]
	if !ok {
		return 0
	}
	in := float64(u.InputTokens) / 1_000_000 * p.InPerM
	out := float64(u.OutputTokens+u.ThoughtTokens) / 1_000_000 * p.OutPerM
	return in + out
}
