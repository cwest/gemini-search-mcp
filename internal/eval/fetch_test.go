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

package eval

import (
	"strings"
	"testing"
)

func TestHTMLToTextStripsTags(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><title>Ignored</title>
<style>.x{color:red}</style>
<script>var a = 1 < 2 && 3 > 2;</script>
</head>
<body>
<h1>Go 1.26.4</h1>
<p>The latest stable release is <b>Go&nbsp;1.26.4</b>.</p>
<!-- a comment that should vanish -->
<p>Download it from go.dev.</p>
</body></html>`

	got := htmlToText(html)

	for _, want := range []string{"Go 1.26.4", "latest stable release", "Download it from go.dev"} {
		if !strings.Contains(got, want) {
			t.Errorf("text missing %q\n---\n%s", want, got)
		}
	}
	for _, bad := range []string{"color:red", "var a", "<p>", "<b>", "a comment that should vanish", "&nbsp;"} {
		if strings.Contains(got, bad) {
			t.Errorf("text should not contain %q\n---\n%s", bad, got)
		}
	}
}

func TestHTMLToTextDecodesEntities(t *testing.T) {
	got := htmlToText(`<p>Fish &amp; Chips cost &lt;$10&gt; &quot;today&quot;</p>`)
	want := `Fish & Chips cost <$10> "today"`
	if got != want {
		t.Errorf("htmlToText = %q, want %q", got, want)
	}
}

func TestHTMLToTextCollapsesWhitespace(t *testing.T) {
	got := htmlToText("<p>one</p>\n\n\n\n<p>two</p>\n   \n<p>three</p>")
	// Blocks separated by at most a blank line; no runs of >2 newlines.
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("whitespace not collapsed: %q", got)
	}
	for _, w := range []string{"one", "two", "three"} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in %q", w, got)
		}
	}
}

func TestCapTextTruncatesAtBudget(t *testing.T) {
	in := strings.Repeat("a", 100)
	got := capText(in, 10)
	if len(got) != 10 {
		t.Errorf("capText len = %d, want 10", len(got))
	}
}

func TestCapTextLeavesShortInputAlone(t *testing.T) {
	in := "short"
	if got := capText(in, 100); got != in {
		t.Errorf("capText = %q, want %q", got, in)
	}
}

func TestCapTextRespectsRuneBoundaries(t *testing.T) {
	// "héllo" — é is 2 bytes (0xC3 0xA9). A byte cap of 2 must not split it.
	in := "héllo"
	got := capText(in, 2)
	if !utf8ValidString(got) {
		t.Errorf("capText split a rune: %q is not valid UTF-8", got)
	}
	if len(got) > 2 {
		t.Errorf("capText len = %d, want <= 2", len(got))
	}
}
