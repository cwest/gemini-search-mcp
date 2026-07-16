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
	"os"
	"testing"
	"time"
)

// TestSearchLive hits the real Gemini grounding API end-to-end through the
// server's own search.Client and asserts that grounding metadata — the synthesized
// answer, cited sources with resolvable URIs, and the web search queries — all
// survive mapResponse into the Result the MCP tool returns. It is skipped unless
// RUN_VERTEX_INTEGRATION=1 and the GOOGLE_* env is configured.
//
// This is the committed proof that the Gemini Enterprise Agent Platform (Vertex)
// grounding path works: run it with GOOGLE_GENAI_USE_VERTEXAI=true,
// GOOGLE_CLOUD_PROJECT, GOOGLE_CLOUD_LOCATION=global, and ADC
// (GOOGLE_APPLICATION_CREDENTIALS) pointed at the project service-account key. The
// same assertions hold for the AI Studio path (GEMINI_API_KEY), so it doubles as
// the parity check between the two providers.
func TestSearchLive(t *testing.T) {
	if os.Getenv("RUN_VERTEX_INTEGRATION") != "1" {
		t.Skip("set RUN_VERTEX_INTEGRATION=1 to run the live Vertex test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	model := os.Getenv("GEMINI_SEARCH_MODEL")
	if model == "" {
		model = "gemini-3.5-flash"
	}
	c, err := New(ctx, model)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r, err := c.Search(ctx, "what is the latest stable Go version")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if r.Answer == "" {
		t.Error("expected non-empty answer")
	}
	// Grounding metadata must reach the caller, not just Google's raw payload.
	if len(r.Queries) == 0 {
		t.Error("expected grounding metadata to populate at least one web search query")
	}
	if len(r.Sources) == 0 {
		t.Fatal("expected at least one grounding source")
	}
	for i, s := range r.Sources {
		if s.URI == "" {
			t.Errorf("Sources[%d] has empty URI: %+v", i, s)
		}
	}
	t.Logf("answer: %s\nqueries: %v\nsources: %d", r.Answer, r.Queries, len(r.Sources))
}
