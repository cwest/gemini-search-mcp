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

// Package search performs Google-Search-grounded answers via Gemini.
package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"google.golang.org/genai"
)

// defaultMaxTries bounds retry attempts for a single search. Four tries with
// exponential backoff is enough to ride out a per-minute on-demand quota blip
// without hanging the MCP caller.
const defaultMaxTries = 4

// defaultMaxElapsed caps total wall-clock time spent retrying one search.
const defaultMaxElapsed = 30 * time.Second

// Source is one cited web result backing the answer. Domain is populated on
// Vertex AI and empty on AI Studio.
type Source struct {
	Title  string `json:"title"`
	Domain string `json:"domain"`
	URI    string `json:"uri"`
}

// Usage captures token counts for a single grounded call (for cost/latency evals).
type Usage struct {
	InputTokens   int32 `json:"input_tokens"`
	OutputTokens  int32 `json:"output_tokens"`
	ThoughtTokens int32 `json:"thought_tokens"`
	TotalTokens   int32 `json:"total_tokens"`
}

// Result is the synthesized answer plus its grounding evidence.
type Result struct {
	Answer  string   `json:"answer"`
	Sources []Source `json:"sources"`
	Queries []string `json:"queries"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Searcher runs a grounded web search. main depends on this interface so the
// MCP layer can be tested with a stub.
type Searcher interface {
	Search(ctx context.Context, query string) (*Result, error)
}

// generateFunc matches genai.Models.GenerateContent. It is a field on Client so
// retry behavior can be tested without a live backend.
type generateFunc func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)

// Client is the genai-backed Searcher.
type Client struct {
	genai *genai.Client
	model string

	// generate performs the underlying GenerateContent call. Defaults to the
	// real genai call; overridden in tests.
	generate generateFunc
	// newBackOff produces the retry schedule for a single search. Defaults to
	// an exponential backoff with jitter; overridden in tests for speed.
	newBackOff func() backoff.BackOff
	// maxTries bounds retry attempts per search.
	maxTries uint
}

// New builds a Client. Backend/project/location/credentials are auto-detected
// from the standard GOOGLE_* environment variables by the genai SDK.
func New(ctx context.Context, model string) (*Client, error) {
	c, err := genai.NewClient(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("genai.NewClient: %w", err)
	}
	return &Client{
		genai:      c,
		model:      model,
		generate:   c.Models.GenerateContent,
		newBackOff: func() backoff.BackOff { return backoff.NewExponentialBackOff() },
		maxTries:   defaultMaxTries,
	}, nil
}

// isRetryable reports whether err is a transient Vertex AI / Gemini error worth
// retrying: rate limits (429) and server errors (500, 503). All other errors —
// including client errors (4xx) and non-API errors — are permanent.
func isRetryable(err error) bool {
	var apiErr *genai.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case 429, 500, 503:
		return true
	default:
		return false
	}
}

// Search runs a grounded GenerateContent call and maps the response. Transient
// failures (429 rate limit, 500/503 server errors) are retried with exponential
// backoff and jitter; all other errors are returned immediately.
func (c *Client) Search(ctx context.Context, query string) (*Result, error) {
	zero := int32(0)
	cfg := &genai.GenerateContentConfig{
		Tools:          []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}},
		ThinkingConfig: &genai.ThinkingConfig{ThinkingBudget: &zero},
	}

	op := func() (*genai.GenerateContentResponse, error) {
		resp, err := c.generate(ctx, c.model, genai.Text(query), cfg)
		if err != nil {
			if isRetryable(err) {
				return nil, err // retry
			}
			return nil, backoff.Permanent(err) // stop immediately
		}
		return resp, nil
	}

	resp, err := backoff.Retry(ctx, op,
		backoff.WithBackOff(c.newBackOff()),
		backoff.WithMaxTries(c.maxTries),
		backoff.WithMaxElapsedTime(defaultMaxElapsed),
	)
	if err != nil {
		return nil, fmt.Errorf("GenerateContent: %w", err)
	}
	return mapResponse(resp), nil
}

// mapResponse extracts the answer, sources, and queries from a genai response.
func mapResponse(resp *genai.GenerateContentResponse) *Result {
	out := &Result{Answer: strings.TrimSpace(resp.Text())}
	if um := resp.UsageMetadata; um != nil {
		out.Usage = &Usage{
			InputTokens:   um.PromptTokenCount,
			OutputTokens:  um.CandidatesTokenCount,
			ThoughtTokens: um.ThoughtsTokenCount,
			TotalTokens:   um.TotalTokenCount,
		}
	}
	if len(resp.Candidates) == 0 {
		return out
	}
	gm := resp.Candidates[0].GroundingMetadata
	if gm == nil {
		return out
	}
	out.Queries = gm.WebSearchQueries
	for _, ch := range gm.GroundingChunks {
		if ch.Web != nil {
			out.Sources = append(out.Sources, Source{
				Title:  ch.Web.Title,
				Domain: ch.Web.Domain,
				URI:    ch.Web.URI,
			})
		}
	}
	return out
}
