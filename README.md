# Example CLI LLM Assistant

Get short and helpful examples and answers with a single CLI command.

# Installation

``` 
go install github.com/rhomel/example-cli-llm/cmd/example@latest

# optional: configure `ef` alias on zsh
example --config-shell-helper zsh >> ~/.zshrc && source ~/.zshrc
```

# Usage

By default all output goes to STDOUT.

```
> example "get pods matching the selector version=my-app"
kubectl get pods -l version=my-app
```

```
> example "replace all instances of foo with bar in all markdown files'
find . -name '*.md' -type f -exec sed -i 's/foo/bar/g' {} +
```

```
> example "current unix time seconds"
date +%s
```

```
> example "what is apple in japanese?"
りんご (ringo)
```

```
> example "how do I use a python venv?"
# new env
python3 -m venv .venv

# activate
source .venv/bin/activate

# install packages
pip install <package>

# deactivate
deactivate
```

```
> example "import data.csv to a new sqlite3 table"
sqlite3 my.db ".mode csv" ".import data.csv my_table"
```

Commands with shell-unfriendly characters can be entered by providing no
arguments:

```
> example
prompt> what's $XDG_CONFIG_HOME used for?
# user-specific configuration files for XDG Base Directory spec.
# $XDG_CONFIG_HOME/<app-name>/
```

```
> example
prompt> what's 25c in f?
77f
```

# Optional Flags

- `-p <profile-id>` or `--profile <profile-id>`: use <profile-id> from the
  configuration
- `-s` or `--select`: provide approximately 3 example lines that are selectable
  with a TUI (arrow keys or j+k keys)
- `--config-shell-helper`: output a shell configuration helper that will create
  a short `ef` alias to call `example --select` and send the selected line to
  the terminal command instead of STDOUT. See Shell Helper for more information.

# Configuration

For all configuration, this tool assumes an OpenAI /chat/completions compatible
API is reachable from the base URL.

While all config is optional, if a usable config cannot be resolved the command
will output the missing configuration to STDERR and exit with code 2.

From highest priority to lowest:

1. Environment Variables (static)
2. `EXAMLE_CLI_CONFIG_COMMAND` (see Environment Variables below for more details)
3. $XDG_CONFIG_HOME/example-cli-llm/settings.json
4. $HOME/.config/example-cli-llm/settings.json

## Environment Variables

- `EXAMPLE_CLI_MODEL`: the model name or id to use
- `EXAMPLE_CLI_API_KEY`: the API key to use per a request
- `EXAMPLE_CLI_API_BASE_URL`: the base URL for the LLM API
- `EXAMPLE_CLI_CONFIG_COMMAND`: a command that outputs a settings.json style configuration on demand
- `EXAMPLE_CLI_SYSTEM_PROMPT_CONTENT`: custom content to use for the system prompt
- `EXAMPLE_CLI_SYSTEM_PROMPT_METHOD`: how to update the system prompt with `EXAMPLE_CLI_SYSTEM_PROMPT_CONTENT` (default: replace)

### `EXAMLE_CLI_CONFIG_COMMAND`

This command is called on each invocation with the expectation that its output
is a `settings.json` compatible format.

For example this may be used to translate an existing LLM's tool configuration
to be reused for this tool:

```bash
export EXAMLE_CLI_CONFIG_COMMAND="jq '{default: {model: .model}}' $HOME/.claude/settings.json"
```

## settings.json Format

Example minimal configuration for locally hosted server (e.g. llama.cpp
llama-server):

```
{
  "default": {
    "api_base_url": "http://localhost:8080"
  }
}
```

Example minimal configuration (e.g. litellm proxy):

```
{
  "default": {
    "model": "claude-haiku",
    "api_key": "api-key",
    "api_base_url": "http://localhost:4000"
  }
}
```

Optional select-mode tuning can also be set in `settings.json`. `choices`
controls how many completions are requested with `--select`, and
`temperature` controls how diverse those completions should be. If omitted,
they default to `3` and `0.9`. `choices_as_system_prompt` is off by default
and works around APIs that ignore multi-choice requests by asking for multiple
options in a single response and then splitting them by line for selection.

```
{
  "default": {
    "model": "claude-haiku",
    "api_key": "api-key",
    "api_base_url": "http://localhost:4000",
    "choices": 3,
    "temperature": 0.9,
    "choices_as_system_prompt": true
  }
}
```

Additional profiles may be configured under the `profiles` key. The profile's
settings replace the default settings when defined and the profile is selected.
Note that even if a custom profile has a complete and valid configuration, if
the default configuraiton is missing required required config the command may
still exit early and complain that the default configuration is incomplete.

Example configuration with a "smart" profile to allow switching to a more
intelligent model on demand with the `--profile` flag:

```
{
  "default": {
    "model": "claude-haiku-4-5",
    "api_key": "api-key",
    "api_base_url": "http://localhost:4000"
  },
  "profiles": {
    "smart": {
      "model": "claude-opus-4-6"
    }
  }
}
```

Configuration with an alternative configuration for different providers:

```
{
  "default": {
    "model": "claude-haiku-4-5",
    "api_key": "anthropic-api-key",
    "api_base_url": "http://localhost:4000"
  },
  "profiles": {
    "gpt": {
      "model": "gpt-5.4-nano",
      "api_key": "openai-api-key",
      "api_base_url": "http://localhost:4000"
    }
  }
}
```

Replace the built-in system prompt with a custom prompt:

```
{
  "default": {
    "system_prompt": [
      {
        "method": "replace",
        "content": "You're a terse and helpful CLI tool example provider..."
      }
    ]
  }
}
```

Append additional preferences to the built-in system prompt:

```
{
  "default": {
    "system_prompt": [
      {
        "method": "append",
        "content": "Prefer BSD command syntax over GNU."
      }
    ]
  }
}
```

Manipulate system prompt on-demand aka poor-man's RAG (WARNING: be careful to sanitize if using an API):

```
{
  "default": {
    "system_prompt": [
      {
        "method": "append",
        "command": "echo \"user's shell: $SHELL\""
      }
    ]
  }
}
```

When multiple system_prompt objects are provided, they are applied
sequentially:

```
{
  "default": {
    "system_prompt": [
      {
        "method": "replace",
        "content": "You're a terse and helpful CLI tool example provider..."
      },
      {
        "method": "append",
        "command": "echo \"user's shell: $SHELL\""
      },
      {
        "method": "append",
        "command": "echo \"current system date: $(date)\""
      }
    ]
  }
}
```

Custom profiles use the default configuration system_prompt if they define no
system_prompt. For now if a profile defines a system_prompt, the default
system_prompt configuration is ignored. (TODO: a special `method` may be added
later to specify additive profile system prompts for easier shared
configuration.)

## Shell Helper

The --config-shell-helper will output a shell configuration command to make
command selection usable without manual copy-paste:

```
> example --config-shell-helper zsh
# add to .zshrc to create 'ef' alias
ef() { print -z "$(example -s "$@")" }
```
