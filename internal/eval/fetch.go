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
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// maxFetchBytes caps how much of a publisher page we keep as readable text.
// Faithfulness only needs enough body to check claims; this also bounds the
// judge prompt size and cost.
const maxFetchBytes = 60_000

// fetchTimeout bounds a single source fetch.
const fetchTimeout = 20 * time.Second

// Fetcher resolves a (possibly redirecting) source URI to readable text and the
// final URL it landed on. The Vertex grounding URIs are redirect shims, so an
// implementation must follow redirects to reach the real publisher page.
type Fetcher interface {
	Fetch(ctx context.Context, uri string) (text string, finalURL string, err error)
}

// HTTPFetcher is the live Fetcher: it GETs the URI, follows redirects, strips
// HTML to text, and caps the size. Its network path is exercised by the
// controller, not by unit tests.
type HTTPFetcher struct {
	client *http.Client
	// maxBytes caps the readable text kept per page; 0 uses maxFetchBytes.
	maxBytes int
}

// NewHTTPFetcher builds an HTTPFetcher with a redirect-following client and a
// per-request timeout.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		client:   &http.Client{Timeout: fetchTimeout},
		maxBytes: maxFetchBytes,
	}
}

// Fetch retrieves uri and returns its readable text plus the final resolved URL.
//
// live path exercised by controller — unit tests cover htmlToText/capText with
// fixtures instead of hitting the network.
func (f *HTTPFetcher) Fetch(ctx context.Context, uri string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return "", "", fmt.Errorf("build request: %w", err)
	}
	// A UA that looks like a browser; some publishers 403 the default Go client.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; gemini-search-mcp-eval/1.0)")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch %s: %w", uri, err)
	}
	defer func() { _ = resp.Body.Close() }()

	finalURL := uri
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", finalURL, fmt.Errorf("fetch %s: status %d", finalURL, resp.StatusCode)
	}

	limit := f.maxBytes
	if limit <= 0 {
		limit = maxFetchBytes
	}
	// Read a little past the limit so capText has bytes to trim to a boundary,
	// then bound the raw read so a huge page can't blow memory.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)*4))
	if err != nil {
		return "", finalURL, fmt.Errorf("read body %s: %w", finalURL, err)
	}
	text := capText(htmlToText(string(raw)), limit)
	return text, finalURL, nil
}

var (
	// RE2 has no backreferences, so script and style are matched separately.
	reScript  = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</\s*script\s*>`)
	reStyle   = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</\s*style\s*>`)
	reComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	// Block-level tags become paragraph breaks so text doesn't run together.
	reBlock = regexp.MustCompile(`(?i)</?(p|div|br|li|ul|ol|h[1-6]|tr|table|section|article|header|footer|blockquote)\b[^>]*>`)
	reTag   = regexp.MustCompile(`(?s)<[^>]+>`)
	// 3+ newlines (with optional intervening blank space) collapse to two.
	reMultiNewline = regexp.MustCompile(`\n[ \t]*\n[ \t\n]*`)
	// Runs of spaces/tabs collapse to one.
	reHorizSpace = regexp.MustCompile(`[ \t]+`)
)

// htmlToText strips HTML to plain readable text: it drops script/style/comments,
// turns block tags into line breaks, removes the remaining tags, decodes HTML
// entities, and collapses runaway whitespace. It is a pragmatic readability
// pass, not a full DOM parse — enough to check claims against page prose.
func htmlToText(s string) string {
	s = reScript.ReplaceAllString(s, " ")
	s = reStyle.ReplaceAllString(s, " ")
	s = reComment.ReplaceAllString(s, " ")
	s = reBlock.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)

	// Normalize whitespace per line, then collapse blank-line runs.
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(reHorizSpace.ReplaceAllString(ln, " "))
	}
	s = strings.Join(lines, "\n")
	s = reMultiNewline.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// capText truncates s to at most maxBytes bytes without splitting a UTF-8 rune.
// maxBytes <= 0 returns s unchanged.
func capText(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// utf8ValidString reports whether s is valid UTF-8 (test helper kept here so the
// test file needs no extra import).
func utf8ValidString(s string) bool { return utf8.ValidString(s) }
