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

// liveModel resolves the model id for a live integration run. It defaults to
// gemini-2.5-flash, which in our testing was served on the regional Vertex
// location us-central1. Model/region availability varies (both 2.5-flash and
// 3.5-flash are generally available across regional and global endpoints;
// preview models tend to require global) and a given location may be quota-dry
// on a given project, so consult the Vertex AI locations documentation and
// override with GEMINI_SEARCH_MODEL as needed.
func liveModel() string {
	if m := os.Getenv("GEMINI_SEARCH_MODEL"); m != "" {
		return m
	}
	return "gemini-2.5-flash"
}

// ensureLiveLocation makes the live test runnable out-of-the-box by defaulting
// GOOGLE_CLOUD_LOCATION to us-central1 when it is unset. The genai SDK reads
// this env var directly to pick the Vertex region. `global` was observed to be
// quota-dry for Gemini on some projects (429 RESOURCE_EXHAUSTED), and the
// default model above is region-served, so us-central1 is the safe default.
// An explicit GOOGLE_CLOUD_LOCATION always wins. t.Setenv restores the prior
// value at the end of the test.
func ensureLiveLocation(t *testing.T) {
	t.Helper()
	if os.Getenv("GOOGLE_CLOUD_LOCATION") == "" {
		t.Setenv("GOOGLE_CLOUD_LOCATION", "us-central1")
	}
}

// runLiveSearch performs one real grounded search in the given mode and asserts
// a non-empty answer with at least one grounding source came back.
func runLiveSearch(t *testing.T, mode config.GroundingMode) {
	t.Helper()
	ensureLiveLocation(t)
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
//
// It runs green out of the box against a Vertex project with Gemini quota:
// GOOGLE_CLOUD_LOCATION defaults to us-central1 (see ensureLiveLocation) and
// GEMINI_SEARCH_MODEL defaults to gemini-2.5-flash (see liveModel), a
// region-served combination. Override either env var to test a different
// region/model. Avoid GOOGLE_CLOUD_LOCATION=global, which was observed to be
// quota-dry on some projects.
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
