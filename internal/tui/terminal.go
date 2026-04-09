package tui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"
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
	if err := render(output, model.View()); err != nil {
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
			if err := render(output, model.View()); err != nil {
				return "", err
			}
		}
	}
}

func render(output io.Writer, view string) error {
	_, err := fmt.Fprint(output, ansiCursorHome, ansiClearScreen, view)
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
