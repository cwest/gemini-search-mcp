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

// Package config loads and validates gemini-search-mcp configuration from the
// environment. The genai SDK reads GOOGLE_* credentials itself; this package
// validates that a usable provider is configured so we fail fast with a clear
// message rather than deep inside an API call.
package config

import (
	"fmt"
	"os"
	"time"
)

const (
	defaultModel   = "gemini-3.1-flash-lite"
	defaultTimeout = 30 * time.Second
)

// Config is the validated runtime configuration.
type Config struct {
	Provider string        // "vertex" or "aistudio"
	Model    string        // Gemini model id
	Timeout  time.Duration // per-search timeout
}

// Load reads the environment and returns a validated Config.
func Load() (*Config, error) {
	c := &Config{Model: defaultModel, Timeout: defaultTimeout}

	switch {
	case truthy(os.Getenv("GOOGLE_GENAI_USE_VERTEXAI")),
		os.Getenv("GOOGLE_CLOUD_PROJECT") != "" && os.Getenv("GOOGLE_CLOUD_LOCATION") != "":
		c.Provider = "vertex"
	case os.Getenv("GOOGLE_API_KEY") != "" || os.Getenv("GEMINI_API_KEY") != "":
		c.Provider = "aistudio"
	default:
		return nil, fmt.Errorf("no Gemini provider configured: set GOOGLE_GENAI_USE_VERTEXAI=true with GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION for Vertex AI, or GOOGLE_API_KEY/GEMINI_API_KEY for AI Studio")
	}

	if m := os.Getenv("GEMINI_SEARCH_MODEL"); m != "" {
		c.Model = m
	}
	if t := os.Getenv("GEMINI_SEARCH_TIMEOUT"); t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return nil, fmt.Errorf("invalid GEMINI_SEARCH_TIMEOUT %q: %w", t, err)
		}
		c.Timeout = d
	}
	return c, nil
}

func truthy(s string) bool {
	switch s {
	case "1", "true", "TRUE", "True", "yes", "on":
		return true
	}
	return false
}
