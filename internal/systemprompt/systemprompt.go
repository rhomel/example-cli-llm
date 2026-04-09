package systemprompt

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rhomel/example-cli-llm/internal/config"
)

type Builder struct {
	RunCommand func(context.Context, string) ([]byte, error)
}

func NewBuilder() Builder {
	return Builder{
		RunCommand: func(ctx context.Context, cmdText string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, "bash", "-lc", cmdText)
			return cmd.Output()
		},
	}
}

func (b Builder) Build(ctx context.Context, builtin string, patches []config.SystemPromptPatch) (string, error) {
	result := strings.TrimSpace(builtin)

	for _, patch := range patches {
		content, err := b.patchContent(ctx, patch)
		if err != nil {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(patch.Method)) {
		case "", "replace":
			result = content
		case "append":
			if strings.TrimSpace(result) == "" {
				result = content
				continue
			}
			if strings.TrimSpace(content) == "" {
				continue
			}
			result = result + "\n\n" + content
		default:
			return "", fmt.Errorf("unsupported system prompt method: %s", patch.Method)
		}
	}

	return strings.TrimSpace(result), nil
}

func (b Builder) patchContent(ctx context.Context, patch config.SystemPromptPatch) (string, error) {
	switch {
	case strings.TrimSpace(patch.Command) != "":
		out, err := b.RunCommand(ctx, patch.Command)
		if err != nil {
			return "", fmt.Errorf("run system prompt command: %w", err)
		}
		return strings.TrimSpace(string(out)), nil
	default:
		return strings.TrimSpace(patch.Content), nil
	}
}
