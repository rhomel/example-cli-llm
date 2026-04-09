package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrefersEnvOverCommandOverFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".config", "example-cli-llm", "settings.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{
		"default": {
			"model": "file-model",
			"api_key": "file-key",
			"api_base_url": "http://file"
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		envModel:         "env-model",
		envConfigCommand: `echo '{"default":{"model":"cmd-model","api_key":"cmd-key","api_base_url":"http://cmd"}}'`,
	}

	loader := NewLoader()
	loader.LookupEnv = func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
	loader.HomeDir = func() (string, error) { return tempDir, nil }

	runtime, err := loader.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if runtime.Model != "env-model" {
		t.Fatalf("Model = %q, want env-model", runtime.Model)
	}
	if runtime.APIKey != "cmd-key" {
		t.Fatalf("APIKey = %q, want cmd-key", runtime.APIKey)
	}
	if runtime.APIBaseURL != "http://cmd" {
		t.Fatalf("APIBaseURL = %q, want http://cmd", runtime.APIBaseURL)
	}
}

func TestResolveAppliesProfileOverlay(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	loader.LookupEnv = func(string) (string, bool) { return "", false }
	loader.HomeDir = func() (string, error) { return "/tmp/none", nil }
	loader.ReadFile = func(string) ([]byte, error) {
		return []byte(`{
			"default": {
				"model": "base-model",
				"api_key": "base-key",
				"api_base_url": "http://base",
				"system_prompt": [{"method":"append","content":"base"}]
			},
			"profiles": {
				"smart": {
					"model": "smart-model"
				}
			}
		}`), nil
	}

	runtime, err := loader.Resolve(context.Background(), "smart")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if runtime.Model != "smart-model" {
		t.Fatalf("Model = %q, want smart-model", runtime.Model)
	}
	if runtime.APIKey != "base-key" || runtime.APIBaseURL != "http://base" {
		t.Fatalf("unexpected inherited config: %+v", runtime)
	}
	if len(runtime.SystemPrompt) != 1 || runtime.SystemPrompt[0].Content != "base" {
		t.Fatalf("system prompt inheritance failed: %+v", runtime.SystemPrompt)
	}
}

func TestResolveProfileSystemPromptReplacesDefaultPromptConfig(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	loader.LookupEnv = func(string) (string, bool) { return "", false }
	loader.HomeDir = func() (string, error) { return "/tmp/none", nil }
	loader.ReadFile = func(string) ([]byte, error) {
		return []byte(`{
			"default": {
				"model": "base-model",
				"api_key": "base-key",
				"api_base_url": "http://base",
				"system_prompt": [{"method":"append","content":"base"}]
			},
			"profiles": {
				"smart": {
					"system_prompt": [{"method":"replace","content":"smart"}]
				}
			}
		}`), nil
	}

	runtime, err := loader.Resolve(context.Background(), "smart")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if len(runtime.SystemPrompt) != 1 || runtime.SystemPrompt[0].Content != "smart" {
		t.Fatalf("system prompt replacement failed: %+v", runtime.SystemPrompt)
	}
}

func TestResolveSupportsTypoedConfigCommandEnvName(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		envConfigCommandTypo: `echo '{"default":{"model":"cmd-model","api_key":"cmd-key","api_base_url":"http://cmd"}}'`,
	}

	loader := NewLoader()
	loader.LookupEnv = func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
	loader.HomeDir = func() (string, error) { return "/tmp/none", nil }
	loader.ReadFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }

	runtime, err := loader.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if runtime.Model != "cmd-model" {
		t.Fatalf("Model = %q, want cmd-model", runtime.Model)
	}
}

func TestResolveReportsMissingConfiguration(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	loader.LookupEnv = func(string) (string, bool) { return "", false }
	loader.HomeDir = func() (string, error) { return "/tmp/none", nil }
	loader.ReadFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }

	_, err := loader.Resolve(context.Background(), "")
	if err == nil {
		t.Fatal("Resolve() error = nil, want missing configuration error")
	}
	if !strings.Contains(err.Error(), "missing configuration") {
		t.Fatalf("Resolve() error = %v, want missing configuration message", err)
	}
}

func TestResolveAllowsLocalServerConfigWithOnlyBaseURL(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	loader.LookupEnv = func(string) (string, bool) { return "", false }
	loader.HomeDir = func() (string, error) { return "/tmp/none", nil }
	loader.ReadFile = func(string) ([]byte, error) {
		return []byte(`{
			"default": {
				"api_base_url": "http://localhost:8080"
			}
		}`), nil
	}

	runtime, err := loader.Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if runtime.APIBaseURL != "http://localhost:8080" {
		t.Fatalf("APIBaseURL = %q", runtime.APIBaseURL)
	}
	if runtime.Model != "" || runtime.APIKey != "" {
		t.Fatalf("unexpected local-server defaults: %+v", runtime)
	}
}

func TestResolvePropagatesFileReadErrors(t *testing.T) {
	t.Parallel()

	loader := NewLoader()
	loader.LookupEnv = func(string) (string, bool) { return "", false }
	loader.HomeDir = func() (string, error) { return "/tmp/none", nil }
	loader.ReadFile = func(string) ([]byte, error) { return nil, errors.New("boom") }

	_, err := loader.Resolve(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Resolve() error = %v, want propagated read error", err)
	}
}
