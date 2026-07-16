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

package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantErr  bool
		wantProv string
		wantMdl  string
		wantTO   time.Duration
		wantMode GroundingMode
	}{
		{
			name:     "vertex via use-vertexai flag",
			env:      map[string]string{"GOOGLE_GENAI_USE_VERTEXAI": "true", "GOOGLE_CLOUD_PROJECT": "p", "GOOGLE_CLOUD_LOCATION": "global"},
			wantProv: "vertex", wantMdl: "gemini-3.1-flash-lite", wantTO: 30 * time.Second, wantMode: GroundingGoogleSearch,
		},
		{
			name:     "vertex via project+location",
			env:      map[string]string{"GOOGLE_CLOUD_PROJECT": "p", "GOOGLE_CLOUD_LOCATION": "us-central1"},
			wantProv: "vertex", wantMdl: "gemini-3.1-flash-lite", wantTO: 30 * time.Second, wantMode: GroundingGoogleSearch,
		},
		{
			name:     "ai studio via GEMINI_API_KEY",
			env:      map[string]string{"GEMINI_API_KEY": "k"},
			wantProv: "aistudio", wantMdl: "gemini-3.1-flash-lite", wantTO: 30 * time.Second, wantMode: GroundingGoogleSearch,
		},
		{
			name:     "vertex wins when both set",
			env:      map[string]string{"GOOGLE_CLOUD_PROJECT": "p", "GOOGLE_CLOUD_LOCATION": "global", "GEMINI_API_KEY": "k"},
			wantProv: "vertex", wantMdl: "gemini-3.1-flash-lite", wantTO: 30 * time.Second, wantMode: GroundingGoogleSearch,
		},
		{
			name:     "model and timeout overrides",
			env:      map[string]string{"GEMINI_API_KEY": "k", "GEMINI_SEARCH_MODEL": "gemini-2.5-flash", "GEMINI_SEARCH_TIMEOUT": "5s"},
			wantProv: "aistudio", wantMdl: "gemini-2.5-flash", wantTO: 5 * time.Second, wantMode: GroundingGoogleSearch,
		},
		{
			name:    "no provider configured is an error",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name:     "default grounding mode is google_search on vertex",
			env:      map[string]string{"GOOGLE_GENAI_USE_VERTEXAI": "true", "GOOGLE_CLOUD_PROJECT": "p", "GOOGLE_CLOUD_LOCATION": "global"},
			wantProv: "vertex", wantMdl: "gemini-3.1-flash-lite", wantTO: 30 * time.Second, wantMode: GroundingGoogleSearch,
		},
		{
			name:     "explicit google_search mode on ai studio",
			env:      map[string]string{"GEMINI_API_KEY": "k", "GEMINI_GROUNDING_MODE": "google_search"},
			wantProv: "aistudio", wantMdl: "gemini-3.1-flash-lite", wantTO: 30 * time.Second, wantMode: GroundingGoogleSearch,
		},
		{
			name:     "enterprise mode on vertex is allowed",
			env:      map[string]string{"GOOGLE_GENAI_USE_VERTEXAI": "true", "GOOGLE_CLOUD_PROJECT": "p", "GOOGLE_CLOUD_LOCATION": "global", "GEMINI_GROUNDING_MODE": "enterprise"},
			wantProv: "vertex", wantMdl: "gemini-3.1-flash-lite", wantTO: 30 * time.Second, wantMode: GroundingEnterprise,
		},
		{
			name:    "enterprise mode on ai studio is an error",
			env:     map[string]string{"GEMINI_API_KEY": "k", "GEMINI_GROUNDING_MODE": "enterprise"},
			wantErr: true,
		},
		{
			name:    "unknown grounding mode is an error",
			env:     map[string]string{"GEMINI_API_KEY": "k", "GEMINI_GROUNDING_MODE": "bogus"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{"GOOGLE_GENAI_USE_VERTEXAI", "GOOGLE_CLOUD_PROJECT", "GOOGLE_CLOUD_LOCATION", "GOOGLE_API_KEY", "GEMINI_API_KEY", "GEMINI_SEARCH_MODEL", "GEMINI_SEARCH_TIMEOUT", "GEMINI_GROUNDING_MODE"} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if got.Provider != tt.wantProv {
				t.Errorf("Provider = %q, want %q", got.Provider, tt.wantProv)
			}
			if got.Model != tt.wantMdl {
				t.Errorf("Model = %q, want %q", got.Model, tt.wantMdl)
			}
			if got.Timeout != tt.wantTO {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.wantTO)
			}
			if got.GroundingMode != tt.wantMode {
				t.Errorf("GroundingMode = %q, want %q", got.GroundingMode, tt.wantMode)
			}
		})
	}
}
