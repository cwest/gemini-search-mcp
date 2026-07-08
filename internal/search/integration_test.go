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

// TestSearchLive hits the real Gemini grounding API. It is skipped unless
// RUN_VERTEX_INTEGRATION=1 and the GOOGLE_* env is configured (e.g. Vertex ADC).
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
	if len(r.Sources) == 0 {
		t.Error("expected at least one source")
	}
	t.Logf("answer: %s\nsources: %d", r.Answer, len(r.Sources))
}
