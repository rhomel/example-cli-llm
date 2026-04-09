package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompleteSendsLiteLLMCompatibleRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "test-model" {
			t.Fatalf("model = %v, want test-model", payload["model"])
		}
		if payload["n"] != float64(3) {
			t.Fatalf("n = %v, want 3", payload["n"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"one"}},{"message":{"content":"two"}},{"message":{"content":"three"}}]}`))
	}))
	defer server.Close()

	client := NewClient()
	got, err := client.Complete(context.Background(), Request{
		BaseURL:      server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		SystemPrompt: "system",
		UserPrompt:   "user",
		N:            3,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(got) != 3 || got[0] != "one" || got[2] != "three" {
		t.Fatalf("Complete() = %#v", got)
	}
}

func TestCompleteDiscoversModelWhenMissingAndAllowsNoAPIKey(t *testing.T) {
	t.Parallel()

	var sawEmptyAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("unexpected authorization during discovery: %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"local-model"}]}`))
		case "/chat/completions":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("unexpected authorization for local request: %q", got)
			}
			sawEmptyAuth = true
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["model"] != "local-model" {
				t.Fatalf("model = %v, want local-model", payload["model"])
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer"}}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient()
	got, err := client.Complete(context.Background(), Request{
		BaseURL:      server.URL,
		Model:        "",
		APIKey:       "",
		SystemPrompt: "system",
		UserPrompt:   "user",
		N:            1,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !sawEmptyAuth {
		t.Fatal("chat/completions was not called")
	}
	if len(got) != 1 || got[0] != "answer" {
		t.Fatalf("Complete() = %#v", got)
	}
}

func TestCompleteReturnsDiscoveryErrorWhenNoModelConfiguredOrExposed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := NewClient()
	_, err := client.Complete(context.Background(), Request{
		BaseURL:      server.URL,
		SystemPrompt: "system",
		UserPrompt:   "user",
		N:            1,
	})
	if err == nil || err.Error() != "discover model: no models returned" {
		t.Fatalf("Complete() error = %v", err)
	}
}

func TestCompleteHandlesSingleChoiceWithoutN(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := payload["n"]; ok {
			t.Fatalf("unexpected n in request: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer"}}]}`))
	}))
	defer server.Close()

	client := NewClient()
	got, err := client.Complete(context.Background(), Request{
		BaseURL:      server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		SystemPrompt: "system",
		UserPrompt:   "user",
		N:            1,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(got) != 1 || got[0] != "answer" {
		t.Fatalf("Complete() = %#v", got)
	}
}

func TestCompleteReturnsAPIErrorMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := NewClient()
	_, err := client.Complete(context.Background(), Request{
		BaseURL:      server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		SystemPrompt: "system",
		UserPrompt:   "user",
		N:            1,
	})
	if err == nil || err.Error() != "chat completions request failed: bad request" {
		t.Fatalf("Complete() error = %v", err)
	}
}
