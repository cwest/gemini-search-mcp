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

// pricing holds Gemini list prices, in USD per 1M tokens. Values are the Vertex
// AI Standard-tier, Global-region rate card (captured 2026-07-22); they may
// drift from the live rate card, so update as needed. Unknown models are treated
// as $0 by CostUSD (see the caveat there).
//
// Note: grounded search also carries a flat per-prompt grounding charge
// ($35 / 1k grounded prompts beyond the free tier) that CostUSD does NOT model —
// it is identical across models, so it does not affect the relative cost ranking
// this table drives. See evals/README.md for the token-vs-grounding cost split.
var pricing = map[string]modelPrice{
	// Original committed-run models. gemini-3.5-flash Global text output is
	// $9.00/1M; the prior $2.50 entry understated it. gemini-3.1-pro-preview
	// Global input is $2.00/1M (prior $1.25 was the 2.5 Pro rate).
	"gemini-3.1-pro-preview": {InPerM: 2.00, OutPerM: 12.00},
	"gemini-3.5-flash":       {InPerM: 1.50, OutPerM: 9.00},
	// Default-model-sweep candidate set. The current default's Global price is
	// $0.25 in / $1.50 out (the prior $0.10/$0.40 entry was the 2.5 Flash-Lite
	// rate). gemini-3.6-flash and gemini-3.5-flash-lite GA'd 2026-07-21.
	"gemini-3.1-flash-lite": {InPerM: 0.25, OutPerM: 1.50},
	"gemini-3.6-flash":      {InPerM: 1.50, OutPerM: 7.50},
	"gemini-3.5-flash-lite": {InPerM: 0.30, OutPerM: 2.50},
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
