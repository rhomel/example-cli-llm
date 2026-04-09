package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"

	"github.com/rhomel/example-cli-llm/internal/app"
)

//go:embed default-system-prompt.md
var builtinPrompt string

func main() {
	application := app.New(builtinPrompt, os.Stdin, os.Stdout, os.Stderr)
	if err := application.Run(context.Background(), os.Args[1:]); err != nil {
		var exitErr app.ExitError
		if !errors.As(err, &exitErr) {
			_, _ = fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if exitErr.Err != nil && exitErr.Err.Error() != "" {
			_, _ = fmt.Fprintln(os.Stderr, exitErr.Err.Error())
			os.Exit(exitErr.Code)
		}
		os.Exit(exitErr.Code)
	}
}
