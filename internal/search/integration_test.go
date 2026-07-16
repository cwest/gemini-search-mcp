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

	"github.com/cwest/gemini-search-mcp/internal/config"
)

// liveModel resolves the model id for a live integration run.
func liveModel() string {
	if m := os.Getenv("GEMINI_SEARCH_MODEL"); m != "" {
		return m
	}
	return "gemini-3.5-flash"
}

// runLiveSearch performs one real grounded search in the given mode and asserts
// a non-empty answer with at least one grounding source came back.
func runLiveSearch(t *testing.T, mode config.GroundingMode) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := New(ctx, liveModel(), mode)
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
	if len(r.Sources) == 0 {
		t.Error("expected at least one grounding source")
	}
	t.Logf("mode=%s answer: %s\nsources: %d", mode, r.Answer, len(r.Sources))
}

// TestSearchLive hits the real Gemini grounding API using the default
// google_search tool. Skipped unless RUN_VERTEX_INTEGRATION=1 and the GOOGLE_*
// env is configured (e.g. Vertex ADC).
func TestSearchLive(t *testing.T) {
	if os.Getenv("RUN_VERTEX_INTEGRATION") != "1" {
		t.Skip("set RUN_VERTEX_INTEGRATION=1 to run the live Vertex test")
	}
	runLiveSearch(t, config.GroundingGoogleSearch)
}

// TestSearchLiveEnterprise hits the real Gemini grounding API using the Web
// Grounding for Enterprise tool (enterpriseWebSearch). It is Vertex-only, so it
// is skipped unless RUN_VERTEX_INTEGRATION=1, GEMINI_GROUNDING_MODE=enterprise,
// and the Vertex GOOGLE_* env / ADC is configured. This proves real grounding
// metadata flows back through the enterprise tool.
func TestSearchLiveEnterprise(t *testing.T) {
	if os.Getenv("RUN_VERTEX_INTEGRATION") != "1" {
		t.Skip("set RUN_VERTEX_INTEGRATION=1 to run the live Vertex test")
	}
	if os.Getenv("GEMINI_GROUNDING_MODE") != string(config.GroundingEnterprise) {
		t.Skip("set GEMINI_GROUNDING_MODE=enterprise to run the live enterprise grounding test")
	}
	runLiveSearch(t, config.GroundingEnterprise)
}
