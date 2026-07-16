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
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	r := &Result{
		Answer: "Go 1.26.4 is the latest.",
		Sources: []Source{
			{Title: "Go Downloads", Domain: "go.dev", URI: "https://x/abc"},
			{Title: "Release notes", Domain: "", URI: "https://x/def"},
		},
		Queries: []string{"latest Go version"},
	}
	want := "Go 1.26.4 is the latest.\n\n" +
		"Sources:\n" +
		"1. go.dev — Go Downloads\n   https://x/abc\n" +
		"2. Release notes\n   https://x/def\n" +
		"\nSearches run: latest Go version\n"
	if got := Format(r); got != want {
		t.Errorf("Format mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestFormatSourceLabel pins the source-label rendering across the two grounding
// provider shapes. AI Studio returns a distinct human Title with an empty Domain;
// the Vertex enterprise path returns the site domain in BOTH Title and Domain
// (and sometimes only a Domain). The label must never render a redundant
// "domain — domain" for the enterprise shape — that was the AI-Studio↔enterprise
// parity gap this covers.
func TestFormatSourceLabel(t *testing.T) {
	tests := []struct {
		name   string
		source Source
		want   string // the "N. <label>" line content, without the numbering prefix
	}{
		{
			name:   "ai studio: distinct title, empty domain",
			source: Source{Title: "Go Downloads", Domain: "", URI: "https://x/a"},
			want:   "Go Downloads",
		},
		{
			name:   "ai studio: title and domain distinct",
			source: Source{Title: "Go Downloads", Domain: "go.dev", URI: "https://x/a"},
			want:   "go.dev — Go Downloads",
		},
		{
			name:   "enterprise: title equals domain (no redundant dupe)",
			source: Source{Title: "youtube.com", Domain: "youtube.com", URI: "https://x/a"},
			want:   "youtube.com",
		},
		{
			name:   "enterprise: domain only, empty title",
			source: Source{Title: "", Domain: "f1miamigp.com", URI: "https://x/a"},
			want:   "f1miamigp.com",
		},
		{
			name:   "fallback: both empty renders the uri",
			source: Source{Title: "", Domain: "", URI: "https://x/a"},
			want:   "https://x/a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Result{Answer: "a", Sources: []Source{tt.source}}
			got := Format(r)
			wantLine := "1. " + tt.want + "\n"
			if !strings.Contains(got, wantLine) {
				t.Errorf("Format label:\n got: %q\nwant substring: %q", got, wantLine)
			}
		})
	}
}

func TestFormatNoSources(t *testing.T) {
	r := &Result{Answer: "No web results were available."}
	want := "No web results were available.\n\n(No sources returned.)\n"
	if got := Format(r); got != want {
		t.Errorf("Format mismatch:\n got: %q\nwant: %q", got, want)
	}
}
