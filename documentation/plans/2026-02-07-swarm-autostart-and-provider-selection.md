# Swarm Auto-Start and Provider Selection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `--start-swarm` CLI flag and interactive provider/model selection when creating myses.

**Architecture:** Add config defaults for provider/model, add CLI flag to auto-start idle myses, modify TUI input flow to prompt for provider/model with defaults.

**Tech Stack:** Go 1.22, Cobra CLI, Bubble Tea TUI, TOML config

---

## Phase 1: Add Default Provider/Model to Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `config.toml`
- Test: `internal/config/config_test.go`

**Step 1: Update config struct**

Add default provider/model fields to `Config` struct in `internal/config/config.go` (around line 15):

```go
type Config struct {
	Swarm     SwarmConfig                `toml:"swarm"`
	Providers map[string]ProviderConfig `toml:"providers"`
	MCP       MCPConfig                  `toml:"mcp"`
}

type SwarmConfig struct {
	MaxMyses        int    `toml:"max_myses"`
	DefaultProvider string `toml:"default_provider"`
	DefaultModel    string `toml:"default_model"`
}
```

**Step 2: Update defaults**

Modify `DefaultConfig()` function (around line 43) to include swarm defaults:

```go
func DefaultConfig() *Config {
	return &Config{
		Swarm: SwarmConfig{
			MaxMyses:        16,
			DefaultProvider: "opencode_zen",
			DefaultModel:    "gpt-5-nano",
		},
		Providers: map[string]ProviderConfig{
			// ... existing providers
		},
		// ... rest
	}
}
```

**Step 3: Update config.toml**

Modify `config.toml` (line 1-2):

```toml
[swarm]
max_myses = 16
default_provider = "opencode_zen"
default_model = "gpt-5-nano"
```

**Step 4: Write test**

Add test to `internal/config/config_test.go`:

```go
func TestLoadDefaultProviderModel(t *testing.T) {
	configContent := `
[swarm]
max_myses = 16
default_provider = "ollama"
default_model = "qwen3:8b"

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:8b"
`
	tmpFile := createTempConfig(t, configContent)
	defer os.Remove(tmpFile)

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Swarm.DefaultProvider != "ollama" {
		t.Errorf("expected default_provider=ollama, got %s", cfg.Swarm.DefaultProvider)
	}

	if cfg.Swarm.DefaultModel != "qwen3:8b" {
		t.Errorf("expected default_model=qwen3:8b, got %s", cfg.Swarm.DefaultModel)
	}
}
```

**Step 5: Run test**

```bash
go test ./internal/config -run TestLoadDefaultProviderModel -v
```

Expected: PASS

**Step 6: Build and verify**

```bash
go build ./cmd/zoea
```

Expected: SUCCESS

**Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go config.toml
git commit -m "feat: add default provider/model to swarm config

- Add DefaultProvider and DefaultModel to SwarmConfig
- Update config.toml with defaults (opencode_zen/gpt-5-nano)
- Add test for loading default provider/model"
```

---

## Phase 2: Add --start-swarm CLI Flag

**Files:**
- Modify: `cmd/zoea/main.go`
- Test: Manual (CLI flag testing)

**Step 1: Add flag definition**

Modify `cmd/zoea/main.go` (around line 20-30) to add flag:

```go
var (
	configPath   string
	debugMode    bool
	offlineMode  bool
	startSwarm   bool  // New flag
)

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "path to config file")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&offlineMode, "offline", false, "run in offline mode (stub MCP server)")
	rootCmd.PersistentFlags().BoolVar(&startSwarm, "start-swarm", false, "auto-start all idle myses on launch")
}
```

**Step 2: Pass flag to TUI**

Modify the `run` function (around line 80-100) to pass flag to TUI:

```go
func run(cmd *cobra.Command, args []string) error {
	// ... existing setup code ...

	// Create TUI
	tuiApp := tui.New(commander, st, eventBus, startSwarm)
	
	// ... rest of setup
}
```

**Step 3: Update TUI constructor**

Modify `internal/tui/app.go` TUI struct and constructor (around line 40-60):

```go
type Model struct {
	// ... existing fields ...
	startSwarm bool  // New field
}

func New(commander *core.Commander, store *store.Store, events <-chan core.Event, startSwarm bool) *tea.Program {
	m := &Model{
		commander:   commander,
		store:       store,
		events:      events,
		startSwarm:  startSwarm,
		// ... rest
	}
	return tea.NewProgram(m, tea.WithAltScreen())
}
```

**Step 4: Implement auto-start logic**

Add auto-start in `Init()` method of `internal/tui/app.go` (around line 120):

```go
func (m *Model) Init() tea.Cmd {
	// Auto-start idle myses if flag enabled
	if m.startSwarm {
		go m.autoStartIdleMyses()
	}

	return tea.Batch(
		listenForEvents(m.events),
		tea.EnterAltScreen,
	)
}

// autoStartIdleMyses starts all myses in idle state
func (m *Model) autoStartIdleMyses() {
	myses := m.commander.ListMyses()
	for _, mysis := range myses {
		if mysis.LastError == nil { // Check state is idle
			_ = m.commander.StartMysis(mysis.ID)
		}
	}
}
```

**Step 5: Test manually**

```bash
# Build
go build ./cmd/zoea

# Test without flag (should not auto-start)
./zoea

# Test with flag (should auto-start idle myses)
./zoea --start-swarm
```

**Step 6: Commit**

```bash
git add cmd/zoea/main.go internal/tui/app.go
git commit -m "feat: add --start-swarm CLI flag

Auto-starts all idle myses on application launch when enabled.
Default: disabled (opt-in behavior)"
```

---

## Phase 3: Interactive Provider/Model Selection in TUI

**Files:**
- Modify: `internal/tui/app.go`
- Test: Manual (TUI interaction)

**Step 1: Add input stages enum**

Add to `internal/tui/app.go` (around line 20):

```go
// Input stages for multi-step mysis creation
type InputStage int

const (
	InputStageName InputStage = iota
	InputStageProvider
	InputStageModel
)
```

**Step 2: Update Model struct**

Modify Model struct (around line 40):

```go
type Model struct {
	// ... existing fields ...
	
	// Multi-stage input for mysis creation
	inputStage      InputStage
	pendingMysisName     string
	pendingMysisProvider string
}
```

**Step 3: Modify input prompt text**

Update `Update()` method to show appropriate prompt based on stage (around line 200-250):

```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.inputMode {
		case InputModeNewMysis:
			if msg.Type == tea.KeyEnter {
				value := strings.TrimSpace(m.textInput.Value())
				
				switch m.inputStage {
				case InputStageName:
					// Name is required
					if value == "" {
						m.err = fmt.Errorf("name cannot be empty")
						return m, nil
					}
					m.pendingMysisName = value
					m.inputStage = InputStageProvider
					m.textInput.SetValue("")
					m.textInput.Placeholder = "Provider (empty for default: " + m.config.Swarm.DefaultProvider + ")"
					return m, nil

				case InputStageProvider:
					m.pendingMysisProvider = value
					
					// If provider is empty, use defaults and create
					if value == "" {
						mysis, err := m.commander.CreateMysis(
							m.pendingMysisName,
							m.config.Swarm.DefaultProvider,
						)
						if err == nil {
							m.err = m.commander.StartMysis(mysis.ID())
						} else {
							m.err = err
						}
						m.resetInput()
						m.refreshMysisList()
						return m, nil
					}
					
					// Provider chosen, ask for model
					m.inputStage = InputStageModel
					m.textInput.SetValue("")
					
					// Get model from config for this provider
					defaultModel := ""
					if providerCfg, ok := m.config.Providers[value]; ok {
						defaultModel = providerCfg.Model
					}
					m.textInput.Placeholder = "Model (default: " + defaultModel + ")"
					return m, nil

				case InputStageModel:
					// Model is required when provider was chosen
					if value == "" {
						m.err = fmt.Errorf("model required when provider is specified")
						return m, nil
					}
					
					mysis, err := m.commander.CreateMysis(
						m.pendingMysisName,
						m.pendingMysisProvider,
					)
					if err == nil {
						// Update model (CreateMysis uses provider's default model)
						// Need to add UpdateMysisModel method or pass model to CreateMysis
						m.err = m.commander.StartMysis(mysis.ID())
					} else {
						m.err = err
					}
					m.resetInput()
					m.refreshMysisList()
					return m, nil
				}
			} else if msg.Type == tea.KeyEsc {
				m.resetInput()
				return m, nil
			}
		// ... rest of key handlers
		}
	// ... rest of message handlers
	}
	
	// Update text input
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}
```

**Step 4: Add resetInput helper**

Add helper method to reset input state (around line 600):

```go
func (m *Model) resetInput() {
	m.inputMode = InputModeNormal
	m.inputStage = InputStageName
	m.pendingMysisName = ""
	m.pendingMysisProvider = ""
	m.textInput.SetValue("")
	m.textInput.Blur()
}
```

**Step 5: Update View to show current stage**

Modify `View()` method to show which stage user is on (around line 400-450):

```go
func (m *Model) View() string {
	// ... existing view code ...
	
	if m.inputMode == InputModeNewMysis {
		stageLabel := ""
		switch m.inputStage {
		case InputStageName:
			stageLabel = "Name"
		case InputStageProvider:
			stageLabel = "Provider (1/2)"
		case InputStageModel:
			stageLabel = "Model (2/2)"
		}
		footer := fmt.Sprintf("[%s] %s", stageLabel, m.textInput.View())
		// ... render footer
	}
	
	// ... rest of view
}
```

**Step 6: Store config in Model**

Modify Model initialization to store config (around line 70):

```go
func New(commander *core.Commander, store *store.Store, events <-chan core.Event, startSwarm bool) *tea.Program {
	// Load config
	cfg, err := config.Load("") // Use default path resolution
	if err != nil {
		// Use defaults if load fails
		cfg = config.DefaultConfig()
	}
	
	m := &Model{
		commander:   commander,
		store:       store,
		events:      events,
		startSwarm:  startSwarm,
		config:      cfg,
		inputStage:  InputStageName,
		// ... rest
	}
	return tea.NewProgram(m, tea.WithAltScreen())
}
```

**Step 7: Add config field to Model**

```go
type Model struct {
	// ... existing fields ...
	config *config.Config
}
```

**Step 8: Handle model parameter in CreateMysis**

Note: `CreateMysis` currently doesn't accept a model parameter. It uses the provider's default model from config. This is actually correct behavior - the provider config already specifies the model.

**Simplify Step 3:** Remove model input stage since provider selection already determines the model from config.

**Revised Step 3 (simpler):**

```go
case InputStageProvider:
	m.pendingMysisProvider = value
	
	// If provider is empty, use defaults
	provider := value
	if provider == "" {
		provider = m.config.Swarm.DefaultProvider
	}
	
	// Validate provider exists
	if _, ok := m.config.Providers[provider]; !ok {
		m.err = fmt.Errorf("provider '%s' not found in config", provider)
		m.resetInput()
		return m, nil
	}
	
	// Create mysis with selected provider
	mysis, err := m.commander.CreateMysis(m.pendingMysisName, provider)
	if err == nil {
		m.err = m.commander.StartMysis(mysis.ID())
	} else {
		m.err = err
	}
	m.resetInput()
	m.refreshMysisList()
	return m, nil
```

**Remove InputStageModel** - not needed since provider determines model.

**Step 9: Test manually**

```bash
go build ./cmd/zoea
./zoea

# Test flow:
# 1. Press 'n'
# 2. Enter name: "test"
# 3. Press Enter
# 4. Enter provider: "" (empty for default)
# 5. Verify mysis created with opencode_zen/gpt-5-nano

# Test custom provider:
# 1. Press 'n'
# 2. Enter name: "test2"
# 3. Press Enter
# 4. Enter provider: "ollama-llama"
# 5. Verify mysis created with ollama/llama3.1:8b
```

**Step 10: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: add interactive provider selection for new myses

Multi-stage input flow:
1. Name (required)
2. Provider (optional, defaults to config default)

Provider selection determines model from config.
Empty provider uses swarm.default_provider."
```

---

## Phase 4: Update Documentation

**Files:**
- Modify: `README.md`

**Step 1: Add CLI flag documentation**

Add to README.md CLI section:

```markdown
## CLI Flags

- `--config <path>` - Path to config file (default: `./config.toml` or `~/.zoea-nova/config.toml`)
- `--debug` - Enable debug logging
- `--offline` - Run in offline mode (stub MCP server)
- `--start-swarm` - Auto-start all idle myses on launch (default: disabled)
```

**Step 2: Document provider selection flow**

Add to README.md:

```markdown
## Creating a Mysis

Press `n` to create a new mysis. You'll be prompted for:

1. **Name** (required) - Unique identifier for the mysis
2. **Provider** (optional) - Leave empty to use default from config, or specify:
   - `ollama` - Local Ollama with qwen3:8b
   - `ollama-llama` - Local Ollama with llama3.1:8b
   - `opencode_zen` - OpenCode Zen with gpt-5-nano
   - `zen-pickle` - OpenCode Zen with big-pickle

The model is determined by the provider's configuration in `config.toml`.
```

**Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document --start-swarm flag and provider selection"
```

---

## Testing Checklist

- [ ] Config loads default provider/model correctly
- [ ] `--start-swarm` flag starts idle myses on launch
- [ ] Creating mysis with empty provider uses defaults
- [ ] Creating mysis with specified provider uses that provider's model
- [ ] Invalid provider name shows error
- [ ] ESC key cancels mysis creation at any stage
- [ ] All existing myses still work after changes

---

## Success Criteria

1. `--start-swarm` flag auto-starts idle myses on launch
2. Config has `default_provider` and `default_model` fields
3. New mysis flow prompts for name and provider
4. Empty provider uses config defaults
5. Specified provider uses that provider's configured model
6. Documentation updated

---

