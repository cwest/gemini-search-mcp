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

package search

import (
	"context"
	"testing"

	"github.com/cenkalti/backoff/v5"
	"google.golang.org/genai"

	"github.com/cwest/gemini-search-mcp/internal/config"
)

// captureClient builds a Client with a given grounding mode whose generate func
// records the config it was called with, so tests can assert which grounding
// tool was wired in.
func captureClient(mode config.GroundingMode, capture *genai.GenerateContentConfig) *Client {
	return &Client{
		model:         "test-model",
		groundingMode: mode,
		generate: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			*capture = *cfg
			return okResponse(), nil
		},
		newBackOff: func() backoff.BackOff { return &backoff.ZeroBackOff{} },
		maxTries:   4,
	}
}

func TestSearchWiresGoogleSearchTool(t *testing.T) {
	var got genai.GenerateContentConfig
	c := captureClient(config.GroundingGoogleSearch, &got)

	if _, err := c.Search(context.Background(), "q"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(got.Tools))
	}
	if got.Tools[0].GoogleSearch == nil {
		t.Errorf("GoogleSearch tool not set; Tools[0] = %+v", got.Tools[0])
	}
	if got.Tools[0].EnterpriseWebSearch != nil {
		t.Errorf("EnterpriseWebSearch tool should be nil in google_search mode")
	}
}

func TestSearchWiresEnterpriseTool(t *testing.T) {
	var got genai.GenerateContentConfig
	c := captureClient(config.GroundingEnterprise, &got)

	if _, err := c.Search(context.Background(), "q"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(got.Tools))
	}
	if got.Tools[0].EnterpriseWebSearch == nil {
		t.Errorf("EnterpriseWebSearch tool not set; Tools[0] = %+v", got.Tools[0])
	}
	if got.Tools[0].GoogleSearch != nil {
		t.Errorf("GoogleSearch tool should be nil in enterprise mode")
	}
}

// TestSearchDefaultsToGoogleSearch verifies a zero-value grounding mode (unset)
// falls back to the google_search tool rather than wiring nothing.
func TestSearchDefaultsToGoogleSearch(t *testing.T) {
	var got genai.GenerateContentConfig
	c := captureClient(config.GroundingMode(""), &got)

	if _, err := c.Search(context.Background(), "q"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got.Tools) != 1 || got.Tools[0].GoogleSearch == nil {
		t.Errorf("empty mode should default to GoogleSearch; Tools = %+v", got.Tools)
	}
}

// TestSearchEnterpriseMapsGroundingMetadata proves the enterprise tool's
// response is parsed by the same mapResponse path: Web Grounding for Enterprise
// returns the identical GroundingChunkWeb{Title, Domain, URI} shape, so sources
// and queries flow through unchanged. This guards the "citation handling works
// for the enterprise response shape too" contract.
func TestSearchEnterpriseMapsGroundingMetadata(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{Parts: []*genai.Part{{Text: "Go 1.26.4 is the latest stable release."}}},
			GroundingMetadata: &genai.GroundingMetadata{
				WebSearchQueries: []string{"latest Go version"},
				GroundingChunks: []*genai.GroundingChunk{
					{Web: &genai.GroundingChunkWeb{Domain: "go.dev", Title: "Go Downloads", URI: "https://vertexaisearch.example/redirect/abc"}},
				},
			},
		}},
	}
	c := &Client{
		model:         "test-model",
		groundingMode: config.GroundingEnterprise,
		generate: func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			return resp, nil
		},
		newBackOff: func() backoff.BackOff { return &backoff.ZeroBackOff{} },
		maxTries:   4,
	}

	r, err := c.Search(context.Background(), "q")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if r.Answer != "Go 1.26.4 is the latest stable release." {
		t.Errorf("Answer = %q", r.Answer)
	}
	if len(r.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(r.Sources))
	}
	if r.Sources[0].Domain != "go.dev" || r.Sources[0].Title != "Go Downloads" ||
		r.Sources[0].URI != "https://vertexaisearch.example/redirect/abc" {
		t.Errorf("Sources[0] = %+v", r.Sources[0])
	}
	if len(r.Queries) != 1 || r.Queries[0] != "latest Go version" {
		t.Errorf("Queries = %v", r.Queries)
	}
}
