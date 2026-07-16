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

package server

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cwest/gemini-search-mcp/internal/search"
)

type stubSearcher struct{}

func (stubSearcher) Search(_ context.Context, query string) (*search.Result, error) {
	return &search.Result{
		Answer:  "stub answer for: " + query,
		Sources: []search.Source{{Title: "Example", Domain: "example.com", URI: "https://example.com"}},
		Queries: []string{query},
	}, nil
}

func TestServerWebSearch(t *testing.T) {
	ctx := context.Background()
	srv := New(stubSearcher{})

	clientT, serverT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	res, err := sess.CallTool(ctx, &mcp.CallToolParams{
		Name:      "web_search",
		Arguments: map[string]any{"query": "hello"},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error result")
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "stub answer for: hello") {
		t.Errorf("unexpected tool text: %q", text)
	}
	// The dual return is spec-mandated: the same grounding metadata that appears
	// in the Markdown content must also survive into structuredContent so machine
	// callers get the schema-validated path. Assert the citation reached it.
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent = %T, want map[string]any", res.StructuredContent)
	}
	if got, _ := sc["answer"].(string); got != "stub answer for: hello" {
		t.Errorf("structuredContent answer = %q", got)
	}
	sources, ok := sc["sources"].([]any)
	if !ok || len(sources) == 0 {
		t.Fatalf("structuredContent sources = %v, want at least one cited source", sc["sources"])
	}
	src0, _ := sources[0].(map[string]any)
	if src0["uri"] != "https://example.com" || src0["domain"] != "example.com" {
		t.Errorf("structuredContent sources[0] = %v, want citation uri+domain preserved", src0)
	}
}
