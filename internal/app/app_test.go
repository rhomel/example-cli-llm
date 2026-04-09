package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rhomel/example-cli-llm/internal/chat"
	"github.com/rhomel/example-cli-llm/internal/config"
)

type configLoaderStub struct {
	runtime config.Runtime
	err     error
}

func (s configLoaderStub) Resolve(context.Context, string) (config.Runtime, error) {
	return s.runtime, s.err
}

type promptBuilderStub struct {
	prompt string
	err    error
}

func (s promptBuilderStub) Build(context.Context, string, []config.SystemPromptPatch) (string, error) {
	return s.prompt, s.err
}

type chatClientStub struct {
	answers []string
	err     error
	req     chat.Request
}

func (s *chatClientStub) Complete(_ context.Context, req chat.Request) ([]string, error) {
	s.req = req
	return s.answers, s.err
}

func TestRunWritesShellHelper(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder
	app := New("builtin", strings.NewReader(""), &stdout, &strings.Builder{})

	if err := app.Run(context.Background(), []string{"--config-shell-helper", "zsh"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `ef() { print -z "$(example -s "$@")" }`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), `alias ee='example'`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunReadsPromptFromArgsAndWritesAnswer(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder
	var stderr strings.Builder
	chatStub := &chatClientStub{answers: []string{"date +%s"}}
	app := Application{
		ConfigLoader:  configLoaderStub{runtime: config.Runtime{Model: "m", APIKey: "k", APIBaseURL: "http://base"}},
		PromptBuilder: promptBuilderStub{prompt: "system"},
		ChatClient:    chatStub,
		BuiltinPrompt: "builtin",
		Stdin:         strings.NewReader(""),
		Stdout:        &stdout,
		Stderr:        &stderr,
	}

	if err := app.Run(context.Background(), []string{"current", "unix", "time"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "date +%s\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if chatStub.req.UserPrompt != "current unix time" || chatStub.req.N != 1 || chatStub.req.Temperature != 0 {
		t.Fatalf("request = %+v", chatStub.req)
	}
}

func TestRunReadsPromptInteractively(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder
	var stderr strings.Builder
	chatStub := &chatClientStub{answers: []string{"answer"}}
	app := Application{
		ConfigLoader:  configLoaderStub{runtime: config.Runtime{Model: "m", APIKey: "k", APIBaseURL: "http://base"}},
		PromptBuilder: promptBuilderStub{prompt: "system"},
		ChatClient:    chatStub,
		BuiltinPrompt: "builtin",
		Stdin:         strings.NewReader("what is apple in japanese?\n"),
		Stdout:        &stdout,
		Stderr:        &stderr,
	}

	if err := app.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "answer\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.String() != "prompt> " {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if chatStub.req.UserPrompt != "what is apple in japanese?" {
		t.Fatalf("request = %+v", chatStub.req)
	}
}

func TestRunUsesSelectMode(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder
	var stderr strings.Builder
	chatStub := &chatClientStub{answers: []string{"one", "two", "three"}}
	app := Application{
		ConfigLoader:  configLoaderStub{runtime: config.Runtime{Model: "m", APIKey: "k", APIBaseURL: "http://base", Choices: 4, Temperature: 0.8}},
		PromptBuilder: promptBuilderStub{prompt: "system"},
		ChatClient:    chatStub,
		BuiltinPrompt: "builtin",
		Stdin:         strings.NewReader("2\n"),
		Stdout:        &stdout,
		Stderr:        &stderr,
	}

	if err := app.Run(context.Background(), []string{"-s", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "two\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if chatStub.req.N != 4 || chatStub.req.Temperature != 0.8 {
		t.Fatalf("request = %+v", chatStub.req)
	}
	if !strings.Contains(stderr.String(), "1. one") || !strings.Contains(stderr.String(), "select> ") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunUsesChoicesAsSystemPromptWorkaroundInSelectMode(t *testing.T) {
	t.Parallel()

	var stdout strings.Builder
	var stderr strings.Builder
	chatStub := &chatClientStub{answers: []string{"one\ntwo\nthree"}}
	app := Application{
		ConfigLoader:  configLoaderStub{runtime: config.Runtime{Model: "m", APIKey: "k", APIBaseURL: "http://base", Choices: 3, Temperature: 0.8, ChoicesAsSystemPrompt: true}},
		PromptBuilder: promptBuilderStub{prompt: "builtin\n<choices-mode>\nend"},
		ChatClient:    chatStub,
		BuiltinPrompt: "builtin",
		Stdin:         strings.NewReader("2\n"),
		Stdout:        &stdout,
		Stderr:        &stderr,
	}

	if err := app.Run(context.Background(), []string{"-s", "prompt"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "two\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if chatStub.req.N != 1 {
		t.Fatalf("request N = %d, want 1", chatStub.req.N)
	}
	if !strings.Contains(chatStub.req.SystemPrompt, "You are in multiple option mode: provide at least 3 options one per a line.") {
		t.Fatalf("system prompt = %q", chatStub.req.SystemPrompt)
	}
	if !strings.Contains(stderr.String(), "1. one") || !strings.Contains(stderr.String(), "3. three") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunWritesConfigErrorsToStderrAndReturnsExit2(t *testing.T) {
	t.Parallel()

	var stderr strings.Builder
	app := Application{
		ConfigLoader:  configLoaderStub{err: errors.New("missing configuration: foo")},
		PromptBuilder: promptBuilderStub{prompt: "system"},
		ChatClient:    &chatClientStub{answers: []string{"answer"}},
		BuiltinPrompt: "builtin",
		Stdin:         strings.NewReader(""),
		Stdout:        &strings.Builder{},
		Stderr:        &stderr,
	}

	err := app.Run(context.Background(), []string{"prompt"})
	var exitErr ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 2 {
		t.Fatalf("Run() error = %#v, want exit 2", err)
	}
	if !strings.Contains(stderr.String(), "missing configuration: foo") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestApplyChoicesMode(t *testing.T) {
	t.Parallel()

	got := applyChoicesMode("a\n<choices-mode>\nb", 4)
	if got != "a\nYou are in multiple option mode: provide at least 4 options one per a line.\nb" {
		t.Fatalf("applyChoicesMode() = %q", got)
	}

	got = applyChoicesMode("a\n<choices-mode>\nb", 0)
	if got != "a\n\nb" {
		t.Fatalf("applyChoicesMode() off = %q", got)
	}
}

func TestSplitChoicesLines(t *testing.T) {
	t.Parallel()

	got := splitChoicesLines([]string{" one \n\n two", "three\n"})
	if len(got) != 3 || got[0] != "one" || got[1] != "two" || got[2] != "three" {
		t.Fatalf("splitChoicesLines() = %#v", got)
	}
}
