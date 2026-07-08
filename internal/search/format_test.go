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

import "testing"

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

func TestFormatNoSources(t *testing.T) {
	r := &Result{Answer: "No web results were available."}
	want := "No web results were available.\n\n(No sources returned.)\n"
	if got := Format(r); got != want {
		t.Errorf("Format mismatch:\n got: %q\nwant: %q", got, want)
	}
}
