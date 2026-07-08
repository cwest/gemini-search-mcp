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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/genai"
)

func TestMapResponse(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "grounding_response.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var resp genai.GenerateContentResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	got := mapResponse(&resp)

	if got.Answer != "Go 1.26.4 is the latest stable release." {
		t.Errorf("Answer = %q", got.Answer)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("Sources len = %d, want 2", len(got.Sources))
	}
	if got.Sources[0].Domain != "go.dev" || got.Sources[0].Title != "Go Downloads" {
		t.Errorf("Sources[0] = %+v", got.Sources[0])
	}
	if got.Sources[0].URI != "https://vertexaisearch.example/redirect/abc" {
		t.Errorf("Sources[0].URI = %q", got.Sources[0].URI)
	}
	if len(got.Queries) != 1 || got.Queries[0] != "latest Go version" {
		t.Errorf("Queries = %v", got.Queries)
	}
	if got.Usage == nil {
		t.Fatalf("Usage = nil, want populated")
	}
	if got.Usage.InputTokens != 12 || got.Usage.OutputTokens != 34 ||
		got.Usage.ThoughtTokens != 5 || got.Usage.TotalTokens != 51 {
		t.Errorf("Usage = %+v", got.Usage)
	}
}

func TestMapResponseNoUsage(t *testing.T) {
	resp := &genai.GenerateContentResponse{}
	if got := mapResponse(resp); got.Usage != nil {
		t.Errorf("Usage = %+v, want nil when metadata absent", got.Usage)
	}
}
