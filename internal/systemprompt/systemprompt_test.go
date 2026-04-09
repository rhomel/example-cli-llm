package systemprompt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rhomel/example-cli-llm/internal/config"
)

func TestBuildUsesBuiltinPromptByDefault(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	got, err := builder.Build(context.Background(), "builtin prompt", nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got != "builtin prompt" {
		t.Fatalf("Build() = %q, want builtin prompt", got)
	}
}

func TestBuildAppliesReplaceAndAppendSequentially(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	got, err := builder.Build(context.Background(), "builtin", []config.SystemPromptPatch{
		{Method: "replace", Content: "replacement"},
		{Method: "append", Content: "extra"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got != "replacement\n\nextra" {
		t.Fatalf("Build() = %q", got)
	}
}

func TestBuildUsesCommandContent(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	builder.RunCommand = func(context.Context, string) ([]byte, error) {
		return []byte("shell-derived\n"), nil
	}

	got, err := builder.Build(context.Background(), "builtin", []config.SystemPromptPatch{
		{Method: "append", Command: "echo shell-derived"},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got != "builtin\n\nshell-derived" {
		t.Fatalf("Build() = %q", got)
	}
}

func TestBuildRejectsUnsupportedMethod(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	_, err := builder.Build(context.Background(), "builtin", []config.SystemPromptPatch{
		{Method: "prepend", Content: "nope"},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported system prompt method") {
		t.Fatalf("Build() error = %v, want unsupported method error", err)
	}
}

func TestBuildPropagatesCommandErrors(t *testing.T) {
	t.Parallel()

	builder := NewBuilder()
	builder.RunCommand = func(context.Context, string) ([]byte, error) {
		return nil, errors.New("boom")
	}

	_, err := builder.Build(context.Background(), "builtin", []config.SystemPromptPatch{
		{Method: "append", Command: "false"},
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Build() error = %v, want command error", err)
	}
}
