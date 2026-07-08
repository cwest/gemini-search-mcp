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

// Command gemini-search-mcp is an MCP stdio server exposing a web_search tool
// backed by Gemini + Google Search grounding (Vertex AI or AI Studio).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/cwest/gemini-search-mcp/internal/config"
	"github.com/cwest/gemini-search-mcp/internal/search"
	"github.com/cwest/gemini-search-mcp/internal/server"
	"github.com/cwest/gemini-search-mcp/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("gemini-search-mcp %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("gemini-search-mcp %s: provider=%s model=%s", version.Version, cfg.Provider, cfg.Model)

	ctx := context.Background()
	sc, err := search.New(ctx, cfg.Model)
	if err != nil {
		log.Fatalf("init search: %v", err)
	}

	srv := server.New(timeoutSearcher{inner: sc, timeout: cfg.Timeout})
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server run: %v", err)
	}
}

// timeoutSearcher wraps a Searcher with a per-call deadline.
type timeoutSearcher struct {
	inner   search.Searcher
	timeout time.Duration
}

func (t timeoutSearcher) Search(ctx context.Context, query string) (*search.Result, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	return t.inner.Search(ctx, query)
}
