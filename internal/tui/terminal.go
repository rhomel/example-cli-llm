package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	ansiEnterAltScreen = "\x1b[?1049h"
	ansiExitAltScreen  = "\x1b[?1049l"
	ansiHideCursor     = "\x1b[?25l"
	ansiShowCursor     = "\x1b[?25h"
	ansiClearScreen    = "\x1b[2J"
	ansiCursorHome     = "\x1b[H"
)

var ErrCancelled = errors.New("selection cancelled")

func SelectList(input *os.File, output io.Writer, items []string) (string, error) {
	if len(items) == 0 {
		return "", errors.New("no answers available to select")
	}

	state, err := makeRaw(input)
	if err != nil {
		return "", err
	}
	defer restoreTerminal(input, state)

	cleanup, err := enterScreen(output)
	if err != nil {
		return "", err
	}
	defer cleanup()

	model := NewListModel(items)
	if err := render(output, input, model.View()); err != nil {
		return "", err
	}

	var buf [3]byte
	for {
		n, err := input.Read(buf[:])
		if err != nil {
			return "", err
		}
		key := ParseKey(buf[:n])
		switch model.Update(key) {
		case ActionAccept:
			return model.Selected(), nil
		case ActionCancel:
			return "", ErrCancelled
		default:
			if err := render(output, input, model.View()); err != nil {
				return "", err
			}
		}
	}
}

func render(output io.Writer, tty *os.File, view string) error {
	height := terminalHeight(tty)
	_, err := fmt.Fprint(output, ansiCursorHome, ansiClearScreen, terminalLines(padViewToBottom(view, height)))
	return err
}

func enterScreen(output io.Writer) (func(), error) {
	if _, err := fmt.Fprint(output, ansiEnterAltScreen, ansiHideCursor, ansiCursorHome, ansiClearScreen); err != nil {
		return nil, err
	}
	return func() {
		_, _ = fmt.Fprint(output, ansiCursorHome, ansiClearScreen, ansiShowCursor, ansiExitAltScreen)
	}, nil
}

type terminalState = term.State

func makeRaw(file *os.File) (*terminalState, error) {
	return term.MakeRaw(int(file.Fd()))
}

func restoreTerminal(file *os.File, state *terminalState) {
	if state == nil {
		return
	}
	_ = term.Restore(int(file.Fd()), state)
}

func terminalHeight(file *os.File) int {
	if file == nil {
		return 0
	}
	_, height, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return 0
	}
	return height
}

func terminalLines(value string) string {
	return strings.ReplaceAll(value, "\n", "\r\n")
}
