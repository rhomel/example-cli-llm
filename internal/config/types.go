package config

type Settings struct {
	Default  ProfileSettings            `json:"default"`
	Profiles map[string]ProfileSettings `json:"profiles"`
}

type ProfileSettings struct {
	Model                 string              `json:"model"`
	APIKey                string              `json:"api_key"`
	APIBaseURL            string              `json:"api_base_url"`
	Choices               *int                `json:"choices"`
	Temperature           *float64            `json:"temperature"`
	ChoicesAsSystemPrompt *bool               `json:"choices_as_system_prompt"`
	SystemPrompt          []SystemPromptPatch `json:"system_prompt"`
}

type SystemPromptPatch struct {
	Method  string `json:"method"`
	Content string `json:"content"`
	Command string `json:"command"`
}

type Runtime struct {
	Model                 string
	APIKey                string
	APIBaseURL            string
	Choices               int
	Temperature           float64
	ChoicesAsSystemPrompt bool
	SystemPrompt          []SystemPromptPatch
}
