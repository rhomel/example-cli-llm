package app

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/rhomel/example-cli-llm/internal/chat"
	"github.com/rhomel/example-cli-llm/internal/config"
	"github.com/rhomel/example-cli-llm/internal/systemprompt"
	"github.com/rhomel/example-cli-llm/internal/tui"
)

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

type Application struct {
	ConfigLoader interface {
		Resolve(context.Context, string) (config.Runtime, error)
	}
	PromptBuilder interface {
		Build(context.Context, string, []config.SystemPromptPatch) (string, error)
	}
	ChatClient interface {
		Complete(context.Context, chat.Request) ([]string, error)
	}
	BuiltinPrompt string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
}

func New(builtinPrompt string, stdin io.Reader, stdout, stderr io.Writer) Application {
	return Application{
		ConfigLoader:  config.NewLoader(),
		PromptBuilder: systemprompt.NewBuilder(),
		ChatClient:    chat.NewClient(),
		BuiltinPrompt: builtinPrompt,
		Stdin:         stdin,
		Stdout:        stdout,
		Stderr:        stderr,
	}
}

func (a Application) Run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("example", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)

	var profile string
	var selectMode bool
	var shellHelper string

	fs.StringVar(&profile, "profile", "", "")
	fs.StringVar(&profile, "p", "", "")
	fs.BoolVar(&selectMode, "select", false, "")
	fs.BoolVar(&selectMode, "s", false, "")
	fs.StringVar(&shellHelper, "config-shell-helper", "", "")

	if err := fs.Parse(args); err != nil {
		return ExitError{Code: 2, Err: err}
	}

	if shellHelper != "" {
		return a.writeShellHelper(shellHelper)
	}

	prompt, err := readPrompt(fs.Args(), a.Stdin, a.Stderr)
	if err != nil {
		return ExitError{Code: 2, Err: err}
	}

	runtime, err := a.ConfigLoader.Resolve(ctx, profile)
	if err != nil {
		return ExitError{Code: 2, Err: writeLine(a.Stderr, err.Error())}
	}

	systemPrompt, err := a.PromptBuilder.Build(ctx, a.BuiltinPrompt, runtime.SystemPrompt)
	if err != nil {
		return err
	}

	n := 1
	temperature := 0.0
	if selectMode {
		n = runtime.Choices
		temperature = runtime.Temperature
		if runtime.ChoicesAsSystemPrompt {
			systemPrompt = applyChoicesMode(systemPrompt, runtime.Choices)
			n = 1
		} else {
			systemPrompt = applyChoicesMode(systemPrompt, 0)
		}
	} else {
		systemPrompt = applyChoicesMode(systemPrompt, 0)
	}

	answers, err := a.ChatClient.Complete(ctx, chat.Request{
		BaseURL:      runtime.APIBaseURL,
		APIKey:       runtime.APIKey,
		Model:        runtime.Model,
		SystemPrompt: systemPrompt,
		UserPrompt:   prompt,
		N:            n,
		Temperature:  temperature,
	})
	if err != nil {
		return err
	}
	if selectMode && runtime.ChoicesAsSystemPrompt {
		answers = splitChoicesLines(answers)
	}

	answer := answers[0]
	if selectMode {
		answer, err = selectAnswer(answers, a.Stdin, a.Stderr)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				return ExitError{Code: 130}
			}
			return ExitError{Code: 2, Err: err}
		}
	}

	return writeLine(a.Stdout, answer)
}

func (a Application) writeShellHelper(shell string) error {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "zsh":
		return writeLine(a.Stdout, "# add to .zshrc to create 'ef' and 'ee' aliases\nef() { print -z \"$(example -s \"$@\")\" }\nalias ee='example'")
	default:
		return ExitError{Code: 2, Err: fmt.Errorf("unsupported shell helper: %s", shell)}
	}
}

func readPrompt(args []string, stdin io.Reader, stderr io.Writer) (string, error) {
	if len(args) > 0 {
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}
	if _, err := fmt.Fprint(stderr, "prompt> "); err != nil {
		return "", err
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", errors.New("prompt is required")
	}
	return line, nil
}

func selectAnswer(answers []string, stdin io.Reader, stderr io.Writer) (string, error) {
	file, ok := stdin.(*os.File)
	if !ok {
		return selectAnswerNumeric(answers, stdin, stderr)
	}
	selected, err := tui.SelectList(file, stderr, answers)
	if err == nil {
		return selected, nil
	}
	if errors.Is(err, tui.ErrCancelled) {
		return "", err
	}
	return selectAnswerNumeric(answers, stdin, stderr)
}

func selectAnswerNumeric(answers []string, stdin io.Reader, stderr io.Writer) (string, error) {
	if len(answers) == 0 {
		return "", errors.New("no answers available to select")
	}
	for i, answer := range answers {
		if _, err := fmt.Fprintf(stderr, "%d. %s\n", i+1, answer); err != nil {
			return "", err
		}
	}
	if _, err := fmt.Fprint(stderr, "select> "); err != nil {
		return "", err
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	index, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || index < 1 || index > len(answers) {
		return "", fmt.Errorf("invalid selection: %s", strings.TrimSpace(line))
	}
	return answers[index-1], nil
}

func writeLine(w io.Writer, value string) error {
	_, err := fmt.Fprintln(w, value)
	return err
}

func applyChoicesMode(systemPrompt string, n int) string {
	replacement := ""
	if n > 0 {
		replacement = fmt.Sprintf("You are in multiple option mode: provide at least %d options one per a line.", n)
	}
	return strings.ReplaceAll(systemPrompt, "<choices-mode>", replacement)
}

func splitChoicesLines(answers []string) []string {
	var split []string
	for _, answer := range answers {
		for _, line := range strings.Split(answer, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			split = append(split, line)
		}
	}
	if len(split) == 0 {
		return answers
	}
	return split
}
