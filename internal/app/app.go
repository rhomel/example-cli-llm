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
	"syscall"
	"unsafe"

	"github.com/rhomel/example-cli-llm/internal/chat"
	"github.com/rhomel/example-cli-llm/internal/config"
	"github.com/rhomel/example-cli-llm/internal/systemprompt"
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
	ConfigLoader  interface{ Resolve(context.Context, string) (config.Runtime, error) }
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

	prompt, err := readPrompt(fs.Args(), a.Stdin, a.Stdout)
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
	if selectMode {
		n = 3
	}

	answers, err := a.ChatClient.Complete(ctx, chat.Request{
		BaseURL:      runtime.APIBaseURL,
		APIKey:       runtime.APIKey,
		Model:        runtime.Model,
		SystemPrompt: systemPrompt,
		UserPrompt:   prompt,
		N:            n,
	})
	if err != nil {
		return err
	}

	answer := answers[0]
	if selectMode {
		answer, err = selectAnswer(answers, a.Stdin, a.Stderr)
		if err != nil {
			return ExitError{Code: 2, Err: err}
		}
	}

	return writeLine(a.Stdout, answer)
}

func (a Application) writeShellHelper(shell string) error {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "zsh":
		return writeLine(a.Stdout, "# add to .zshrc to create 'ef' alias\nef() { print -z \"$(example -s \"$@\")\" }")
	default:
		return ExitError{Code: 2, Err: fmt.Errorf("unsupported shell helper: %s", shell)}
	}
}

func readPrompt(args []string, stdin io.Reader, stdout io.Writer) (string, error) {
	if len(args) > 0 {
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}
	if _, err := fmt.Fprint(stdout, "prompt> "); err != nil {
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
	if file, ok := stdin.(*os.File); ok {
		if selected, err := selectAnswerInteractive(answers, file, stderr); err == nil {
			return selected, nil
		}
	}
	return selectAnswerNumeric(answers, stdin, stderr)
}

func selectAnswerInteractive(answers []string, stdin *os.File, stderr io.Writer) (string, error) {
	if len(answers) == 0 {
		return "", errors.New("no answers available to select")
	}
	state, err := makeRaw(stdin)
	if err != nil {
		return "", err
	}
	defer restoreTerminal(stdin, state)

	selected := 0
	if err := renderSelection(stderr, answers, selected); err != nil {
		return "", err
	}

	var buf [3]byte
	for {
		n, err := stdin.Read(buf[:])
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		switch {
		case buf[0] == '\r' || buf[0] == '\n':
			if _, err := fmt.Fprint(stderr, "\n"); err != nil {
				return "", err
			}
			return answers[selected], nil
		case buf[0] == 'j':
			selected = (selected + 1) % len(answers)
		case buf[0] == 'k':
			selected = (selected - 1 + len(answers)) % len(answers)
		case n >= 3 && buf[0] == 27 && buf[1] == 91 && buf[2] == 65:
			selected = (selected - 1 + len(answers)) % len(answers)
		case n >= 3 && buf[0] == 27 && buf[1] == 91 && buf[2] == 66:
			selected = (selected + 1) % len(answers)
		default:
			continue
		}
		if err := renderSelection(stderr, answers, selected); err != nil {
			return "", err
		}
	}
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

func renderSelection(stderr io.Writer, answers []string, selected int) error {
	if _, err := fmt.Fprint(stderr, "\x1b[H\x1b[2J"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(stderr, "select with arrow keys or j/k, press enter"); err != nil {
		return err
	}
	for i, answer := range answers {
		prefix := "  "
		if i == selected {
			prefix = "> "
		}
		if _, err := fmt.Fprintf(stderr, "%s%s\n", prefix, answer); err != nil {
			return err
		}
	}
	return nil
}

type terminalState struct {
	value syscall.Termios
}

func makeRaw(file *os.File) (*terminalState, error) {
	termios, err := readTermios(file.Fd())
	if err != nil {
		return nil, err
	}
	state := &terminalState{value: *termios}
	raw := *termios
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Iflag &^= syscall.ICRNL | syscall.INLCR | syscall.IXON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := writeTermios(file.Fd(), &raw); err != nil {
		return nil, err
	}
	return state, nil
}

func restoreTerminal(file *os.File, state *terminalState) {
	if state == nil {
		return
	}
	_ = writeTermios(file.Fd(), &state.value)
}

func readTermios(fd uintptr) (*syscall.Termios, error) {
	termios := &syscall.Termios{}
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return termios, nil
}

func writeTermios(fd uintptr, termios *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
