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
	"fmt"
	"strings"
)

// Format renders a Result as the markdown text returned over MCP.
func Format(r *Result) string {
	var b strings.Builder
	b.WriteString(r.Answer)
	if len(r.Sources) == 0 {
		b.WriteString("\n\n(No sources returned.)\n")
		return b.String()
	}
	b.WriteString("\n\nSources:\n")
	for i, s := range r.Sources {
		label := s.Title
		if s.Domain != "" {
			label = s.Domain + " — " + s.Title
		}
		fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, label, s.URI)
	}
	if len(r.Queries) > 0 {
		fmt.Fprintf(&b, "\nSearches run: %s\n", strings.Join(r.Queries, ", "))
	}
	return b.String()
}
