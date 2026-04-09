package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	envModel              = "EXAMPLE_CLI_MODEL"
	envAPIKey             = "EXAMPLE_CLI_API_KEY"
	envAPIBaseURL         = "EXAMPLE_CLI_API_BASE_URL"
	envConfigCommand      = "EXAMPLE_CLI_CONFIG_COMMAND"
	envConfigCommandTypo  = "EXAMLE_CLI_CONFIG_COMMAND"
	envSystemPrompt       = "EXAMPLE_CLI_SYSTEM_PROMPT_CONTENT"
	envSystemPromptMethod = "EXAMPLE_CLI_SYSTEM_PROMPT_METHOD"
)

type Loader struct {
	LookupEnv func(string) (string, bool)
	HomeDir   func() (string, error)
	ReadFile  func(string) ([]byte, error)
	RunConfig func(context.Context, string) ([]byte, error)
}

func NewLoader() Loader {
	return Loader{
		LookupEnv: os.LookupEnv,
		HomeDir:   os.UserHomeDir,
		ReadFile:  os.ReadFile,
		RunConfig: func(ctx context.Context, cmdText string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, "bash", "-lc", cmdText)
			return cmd.Output()
		},
	}
}

func (l Loader) Resolve(ctx context.Context, profile string) (Runtime, error) {
	settings, err := l.loadSettings(ctx)
	if err != nil {
		return Runtime{}, err
	}

	resolved := mergeProfile(settings.Default, settings.Profiles[profile], profile != "")
	resolved = applyEnvOverrides(resolved, l.LookupEnv)

	runtime := Runtime{
		Model:        strings.TrimSpace(resolved.Model),
		APIKey:       strings.TrimSpace(resolved.APIKey),
		APIBaseURL:   strings.TrimRight(strings.TrimSpace(resolved.APIBaseURL), "/"),
		SystemPrompt: resolved.SystemPrompt,
	}
	if err := validateRuntime(runtime); err != nil {
		return Runtime{}, err
	}
	return runtime, nil
}

func (l Loader) loadSettings(ctx context.Context) (Settings, error) {
	var merged Settings

	fileSettings, err := l.loadFileSettings()
	if err != nil {
		return Settings{}, err
	}
	merged = mergeSettings(merged, fileSettings)

	cmdText := firstEnv(l.LookupEnv, envConfigCommand, envConfigCommandTypo)
	if cmdText != "" {
		out, err := l.RunConfig(ctx, cmdText)
		if err != nil {
			return Settings{}, fmt.Errorf("run config command: %w", err)
		}
		cmdSettings, err := parseSettings(out)
		if err != nil {
			return Settings{}, fmt.Errorf("parse config command output: %w", err)
		}
		merged = mergeSettings(merged, cmdSettings)
	}

	return merged, nil
}

func (l Loader) loadFileSettings() (Settings, error) {
	paths, err := l.configPaths()
	if err != nil {
		return Settings{}, err
	}
	for _, path := range paths {
		data, err := l.ReadFile(path)
		if err == nil {
			return parseSettings(data)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return Settings{}, fmt.Errorf("read config %s: %w", path, err)
		}
	}
	return Settings{}, nil
}

func (l Loader) configPaths() ([]string, error) {
	homeDir, err := l.HomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}
	if xdg, ok := l.LookupEnv("XDG_CONFIG_HOME"); ok && strings.TrimSpace(xdg) != "" {
		return []string{
			filepath.Join(strings.TrimSpace(xdg), "example-cli-llm", "settings.json"),
			filepath.Join(homeDir, ".config", "example-cli-llm", "settings.json"),
		}, nil
	}
	return []string{
		filepath.Join(homeDir, ".config", "example-cli-llm", "settings.json"),
	}, nil
}

func parseSettings(data []byte) (Settings, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return Settings{}, nil
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
	}
	if settings.Profiles == nil {
		settings.Profiles = map[string]ProfileSettings{}
	}
	return settings, nil
}

func mergeSettings(base, overlay Settings) Settings {
	if overlay.ModelPresent() {
		base.Default = mergeProfile(base.Default, overlay.Default, true)
	}
	if base.Profiles == nil {
		base.Profiles = map[string]ProfileSettings{}
	}
	for name, profile := range overlay.Profiles {
		base.Profiles[name] = mergeProfile(base.Profiles[name], profile, true)
	}
	return base
}

func (s Settings) ModelPresent() bool {
	return s.Default.Model != "" || s.Default.APIKey != "" || s.Default.APIBaseURL != "" || len(s.Default.SystemPrompt) > 0 || len(s.Profiles) > 0
}

func mergeProfile(base, overlay ProfileSettings, replaceSystemPrompt bool) ProfileSettings {
	if overlay.Model != "" {
		base.Model = overlay.Model
	}
	if overlay.APIKey != "" {
		base.APIKey = overlay.APIKey
	}
	if overlay.APIBaseURL != "" {
		base.APIBaseURL = overlay.APIBaseURL
	}
	if replaceSystemPrompt && len(overlay.SystemPrompt) > 0 {
		base.SystemPrompt = append([]SystemPromptPatch(nil), overlay.SystemPrompt...)
	}
	return base
}

func applyEnvOverrides(profile ProfileSettings, lookupEnv func(string) (string, bool)) ProfileSettings {
	if value, ok := lookupEnv(envModel); ok && strings.TrimSpace(value) != "" {
		profile.Model = value
	}
	if value, ok := lookupEnv(envAPIKey); ok && strings.TrimSpace(value) != "" {
		profile.APIKey = value
	}
	if value, ok := lookupEnv(envAPIBaseURL); ok && strings.TrimSpace(value) != "" {
		profile.APIBaseURL = value
	}
	if value, ok := lookupEnv(envSystemPrompt); ok && strings.TrimSpace(value) != "" {
		method := "replace"
		if envMethod, ok := lookupEnv(envSystemPromptMethod); ok && strings.TrimSpace(envMethod) != "" {
			method = envMethod
		}
		profile.SystemPrompt = []SystemPromptPatch{{
			Method:  method,
			Content: value,
		}}
	}
	return profile
}

func firstEnv(lookupEnv func(string) (string, bool), keys ...string) string {
	for _, key := range keys {
		if value, ok := lookupEnv(key); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func validateRuntime(runtime Runtime) error {
	var missing []string
	if runtime.APIBaseURL == "" {
		missing = append(missing, envAPIBaseURL+" or settings.default.api_base_url")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing configuration: %s", strings.Join(missing, ", "))
}
