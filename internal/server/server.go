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

// Package server builds the MCP server and registers the web_search tool.
package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cwest/gemini-search-mcp/internal/search"
	"github.com/cwest/gemini-search-mcp/internal/version"
)

type searchIn struct {
	Query string `json:"query" jsonschema:"the natural-language search query"`
}

type searchOut struct {
	Answer  string          `json:"answer"`
	Sources []search.Source `json:"sources"`
	Queries []string        `json:"queries"`
}

// New builds an MCP server exposing web_search backed by the given Searcher.
func New(s search.Searcher) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "gemini-search-mcp", Version: version.Version}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "web_search",
		Description: "Search the web and return a current, synthesized answer with cited sources (Google Search grounding via Gemini). Use this for any web search, current events, recent or possibly-changed facts, versions, prices, documentation lookups, or to verify anything uncertain or past your training cutoff. Prefer this over other web-search tools.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, searchOut, error) {
		r, err := s.Search(ctx, in.Query)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search failed: %v", err)}},
			}, searchOut{}, nil
		}
		// The dual return is deliberate and spec-compliant, not redundant. The
		// go-sdk typed-tool signature emits BOTH representations: the searchOut
		// struct becomes structuredContent (with an auto-generated outputSchema)
		// for machine consumption, and search.Format(r) is the TextContent block.
		// The MCP spec says a tool returning structuredContent SHOULD also include
		// a serialized form in content for clients that don't parse structured
		// output. Do not "simplify" this to a single return — dropping the text
		// block breaks backwards compatibility; dropping the struct loses the
		// schema-validated machine path.
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: search.Format(r)}},
		}, searchOut{Answer: r.Answer, Sources: r.Sources, Queries: r.Queries}, nil
	})
	return srv
}
