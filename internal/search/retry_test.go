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
	"errors"
	"testing"

	"github.com/cenkalti/backoff/v5"
	"google.golang.org/genai"
)

// okResponse is a minimal valid grounded response used by retry tests.
func okResponse() *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content: &genai.Content{Parts: []*genai.Part{{Text: "answer"}}},
		}},
	}
}

// newTestClient builds a Client with a stubbed generateFunc and a zero-delay
// backoff so retry tests run instantly.
func newTestClient(gen generateFunc) *Client {
	return &Client{
		model:      "test-model",
		generate:   gen,
		newBackOff: func() backoff.BackOff { return &backoff.ZeroBackOff{} },
		maxTries:   4,
	}
}

func TestSearchRetriesTransient(t *testing.T) {
	calls := 0
	c := newTestClient(func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		calls++
		if calls < 3 {
			return nil, &genai.APIError{Code: 429, Message: "Resource exhausted"}
		}
		return okResponse(), nil
	})

	got, err := c.Search(context.Background(), "q")
	if err != nil {
		t.Fatalf("Search returned error after transient retries: %v", err)
	}
	if got == nil || got.Answer != "answer" {
		t.Fatalf("Search result = %+v, want answer populated", got)
	}
	if calls != 3 {
		t.Errorf("generate called %d times, want 3 (2 failures + 1 success)", calls)
	}
}

func TestSearchDoesNotRetryPermanent(t *testing.T) {
	calls := 0
	c := newTestClient(func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		calls++
		return nil, &genai.APIError{Code: 400, Message: "bad request"}
	})

	_, err := c.Search(context.Background(), "q")
	if err == nil {
		t.Fatal("Search returned nil error for permanent (400) failure")
	}
	if calls != 1 {
		t.Errorf("generate called %d times for permanent error, want 1", calls)
	}
}

func TestSearchExhaustsRetries(t *testing.T) {
	calls := 0
	c := newTestClient(func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
		calls++
		return nil, &genai.APIError{Code: 503, Message: "unavailable"}
	})

	_, err := c.Search(context.Background(), "q")
	if err == nil {
		t.Fatal("Search returned nil error when retries should be exhausted")
	}
	if calls != 4 {
		t.Errorf("generate called %d times, want 4 (maxTries)", calls)
	}
}

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"429", &genai.APIError{Code: 429}, true},
		{"500", &genai.APIError{Code: 500}, true},
		{"503", &genai.APIError{Code: 503}, true},
		{"400", &genai.APIError{Code: 400}, false},
		{"401", &genai.APIError{Code: 401}, false},
		{"403", &genai.APIError{Code: 403}, false},
		{"404", &genai.APIError{Code: 404}, false},
		{"non-api error", errors.New("boom"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryable(tc.err); got != tc.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
