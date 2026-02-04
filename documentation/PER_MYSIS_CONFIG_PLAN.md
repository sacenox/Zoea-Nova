# Per-Mysis Configuration Implementation Plan

**Date:** 2026-02-04  
**Status:** Ready for implementation

## Goal

Enable per-mysis provider/model/temperature configuration and document OpenCode Zen free models.

## Architecture

The current singleton provider pattern will be replaced with a factory pattern. Each mysis will own a dedicated provider instance configured with its stored model and temperature. Config defines defaults; database stores per-mysis overrides.

## Tech Stack

Go 1.22+, SQLite (modernc.org/sqlite), go-openai, BurntSushi/toml

---

## Phase 1: Documentation

**Files:**
- Create: `documentation/OPENCODE_ZEN_MODELS.md`

**Content:**
```markdown
# OpenCode Zen Models

Models available through OpenCode Zen for use with Zoea Nova.

## Free Models

These models are free during their beta period:

| Model | Model ID | Endpoint | Tool Support |
|-------|----------|----------|--------------|
| GLM 4.7 Free | `glm-4.7-free` | `/v1/chat/completions` | Yes |
| Kimi K2.5 Free | `kimi-k2.5-free` | `/v1/chat/completions` | Yes |
| MiniMax M2.1 Free | `minimax-m2.1-free` | `/v1/messages` | Yes |
| Big Pickle | `big-pickle` | `/v1/chat/completions` | Yes |
| GPT 5 Nano | `gpt-5-nano` | `/v1/responses` | Yes |

## Configuration

```toml
[providers.opencode_zen]
endpoint = "https://api.opencode.ai/v1"
model = "glm-4.7-free"
temperature = 0.7
```

Credentials are stored in `~/.zoea-nova/credentials.json`.

## References

- [OpenCode Zen Documentation](https://opencode.ai/docs/zen/)
- [Model List API](https://opencode.ai/zen/v1/models)
```

---

## Phase 2: Config Changes

**Files:**
- Modify: `internal/config/config.go`
- Modify: `config.toml`

**Changes to `config.go`:**

```go
// ProviderConfig holds LLM provider settings.
type ProviderConfig struct {
    Endpoint    string  `toml:"endpoint"`
    Model       string  `toml:"model"`
    Temperature float64 `toml:"temperature"`
}

// In DefaultConfig():
Providers: map[string]ProviderConfig{
    "ollama": {
        Endpoint:    "http://localhost:11434",
        Model:       "llama3",
        Temperature: 0.7,
    },
},

// In applyEnvOverrides():
if v := os.Getenv("ZOEA_OLLAMA_TEMPERATURE"); v != "" {
    if f, err := strconv.ParseFloat(v, 64); err == nil {
        if p, ok := cfg.Providers["ollama"]; ok {
            p.Temperature = f
            cfg.Providers["ollama"] = p
        }
    }
}

if v := os.Getenv("ZOEA_OPENCODE_TEMPERATURE"); v != "" {
    if f, err := strconv.ParseFloat(v, 64); err == nil {
        if p, ok := cfg.Providers["opencode_zen"]; ok {
            p.Temperature = f
            cfg.Providers["opencode_zen"] = p
        }
    }
}
```

**Changes to `config.toml`:**

```toml
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7

[providers.opencode_zen]
endpoint = "https://api.opencode.ai/v1"
model = "glm-4.7-free"
temperature = 0.7
```

---

## Phase 3: Store Changes

**Files:**
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/myses.go`

**Changes to `schema.sql`:**

```sql
INSERT OR REPLACE INTO schema_version (version) VALUES (4);

CREATE TABLE IF NOT EXISTS myses (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    temperature REAL NOT NULL DEFAULT 0.7,
    state TEXT NOT NULL DEFAULT 'idle',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Changes to `myses.go`:**

```go
// Mysis struct
type Mysis struct {
    ID          string
    Name        string
    Provider    string
    Model       string
    Temperature float64
    State       MysisState
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// CreateMysis signature
func (s *Store) CreateMysis(name, provider, model string, temperature float64) (*Mysis, error)

// UpdateMysisConfig signature
func (s *Store) UpdateMysisConfig(id, provider, model string, temperature float64) error

// Scan functions updated to include temperature
```

---

## Phase 4: Provider Changes

**Files:**
- Modify: `internal/provider/ollama.go`
- Modify: `internal/provider/opencode.go`
- Modify: `internal/provider/provider.go`
- Create: `internal/provider/factory.go`
- Modify: `internal/provider/mock.go`

**Changes to `provider.go`:**

```go
// ProviderFactory creates provider instances with specific configurations.
type ProviderFactory interface {
    Name() string
    Create(model string, temperature float64) Provider
}

// Registry holds provider factories.
type Registry struct {
    factories map[string]ProviderFactory
}

func (r *Registry) RegisterFactory(f ProviderFactory) {
    r.factories[f.Name()] = f
}

func (r *Registry) Create(name, model string, temperature float64) (Provider, error) {
    f, ok := r.factories[name]
    if !ok {
        return nil, ErrProviderNotFound
    }
    return f.Create(model, temperature), nil
}
```

**New `factory.go`:**

```go
package provider

// OllamaFactory creates Ollama providers.
type OllamaFactory struct {
    endpoint string
}

func NewOllamaFactory(endpoint string) *OllamaFactory {
    return &OllamaFactory{endpoint: endpoint}
}

func (f *OllamaFactory) Name() string { return "ollama" }

func (f *OllamaFactory) Create(model string, temperature float64) Provider {
    return NewOllamaWithTemp(f.endpoint, model, temperature)
}

// OpenCodeFactory creates OpenCode Zen providers.
type OpenCodeFactory struct {
    endpoint string
    apiKey   string
}

func NewOpenCodeFactory(endpoint, apiKey string) *OpenCodeFactory {
    return &OpenCodeFactory{endpoint: endpoint, apiKey: apiKey}
}

func (f *OpenCodeFactory) Name() string { return "opencode_zen" }

func (f *OpenCodeFactory) Create(model string, temperature float64) Provider {
    return NewOpenCodeWithTemp(f.endpoint, model, f.apiKey, temperature)
}
```

**Changes to `ollama.go`:**

```go
type OllamaProvider struct {
    client      *openai.Client
    model       string
    temperature float64
}

func NewOllamaWithTemp(endpoint, model string, temperature float64) *OllamaProvider {
    config := openai.DefaultConfig("")
    config.BaseURL = endpoint + "/v1"
    return &OllamaProvider{
        client:      openai.NewClientWithConfig(config),
        model:       model,
        temperature: temperature,
    }
}

// In Chat/ChatWithTools, add Temperature to request:
resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:       p.model,
    Messages:    toOpenAIMessages(messages),
    Temperature: float32(p.temperature),
})
```

**Changes to `opencode.go`:**

Similar temperature field and usage as ollama.go.

---

## Phase 5: Commander Changes

**Files:**
- Modify: `internal/core/commander.go`

**Key changes:**

```go
// Commander now uses factory pattern
type Commander struct {
    // ... existing fields
    registry *provider.Registry  // Now holds factories
}

// CreateMysis creates provider using factory
func (c *Commander) CreateMysis(name, providerName string) (*Mysis, error) {
    provCfg := c.config.Providers[providerName]
    
    // Create provider instance for this mysis
    p, err := c.registry.Create(providerName, provCfg.Model, provCfg.Temperature)
    if err != nil {
        return nil, err
    }
    
    // Store with model and temperature
    stored, err := c.store.CreateMysis(name, providerName, provCfg.Model, provCfg.Temperature)
    // ...
}

// ConfigureMysis now accepts model parameter
func (c *Commander) ConfigureMysis(id, providerName, model string) error {
    provCfg := c.config.Providers[providerName]
    temperature := provCfg.Temperature
    
    // Create new provider with specified model
    p, err := c.registry.Create(providerName, model, temperature)
    // ...
    
    c.store.UpdateMysisConfig(id, providerName, model, temperature)
    mysis.SetProvider(p)
}

// LoadMyses creates providers from stored config
func (c *Commander) LoadMyses() error {
    for _, sm := range stored {
        p, err := c.registry.Create(sm.Provider, sm.Model, sm.Temperature)
        // ...
    }
}
```

---

## Phase 6: TUI Changes

**Files:**
- Modify: `internal/tui/input.go`
- Modify: `internal/tui/app.go`

**Changes to `input.go`:**

```go
const (
    InputModeNone InputMode = iota
    InputModeBroadcast
    InputModeMessage
    InputModeNewMysis
    InputModeConfigProvider
    InputModeConfigModel  // NEW
)

// In SetMode():
case InputModeConfigModel:
    m.textInput.Placeholder = "Enter model name..."
    m.textInput.Prompt = inputPromptStyle.Render("🔧 ") + " "
```

**Changes to `app.go`:**

```go
// Add field to track pending provider during config flow
type Model struct {
    // ... existing fields
    pendingProvider string  // Temporary storage during config flow
}

// In handleInputKey, case InputModeConfigProvider:
case InputModeConfigProvider:
    // Validate provider exists
    if value != "ollama" && value != "opencode_zen" {
        m.err = fmt.Errorf("unknown provider: %s", value)
        m.input.Reset()
        return m, nil
    }
    // Store provider and prompt for model
    m.pendingProvider = value
    m.input.SetMode(InputModeConfigModel, m.input.TargetID())
    return m, m.input.Focus()

case InputModeConfigModel:
    m.err = m.commander.ConfigureMysis(m.input.TargetID(), m.pendingProvider, value)
    m.pendingProvider = ""
```

---

## Phase 7: Main.go Changes

**Files:**
- Modify: `cmd/zoea/main.go`

**Changes:**

```go
func initProviders(cfg *config.Config, creds *config.Credentials) *provider.Registry {
    registry := provider.NewRegistry()

    // Register Ollama factory
    if ollCfg, ok := cfg.Providers["ollama"]; ok {
        factory := provider.NewOllamaFactory(ollCfg.Endpoint)
        registry.RegisterFactory(factory)
    }

    // Register OpenCode Zen factory
    if zenCfg, ok := cfg.Providers["opencode_zen"]; ok {
        apiKey := creds.GetAPIKey("opencode_zen")
        if apiKey != "" {
            factory := provider.NewOpenCodeFactory(zenCfg.Endpoint, apiKey)
            registry.RegisterFactory(factory)
        }
    }

    return registry
}
```

---

## Phase 8: Tests

**Files:**
- Modify: `internal/config/config_test.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/core/commander_test.go`

**Test additions:**

```go
// config_test.go
func TestLoadWithTemperature(t *testing.T) {
    content := `
[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.5
`
    // ... verify temperature is parsed
}

// store_test.go
func TestMysisCRUDWithTemperature(t *testing.T) {
    mysis, err := s.CreateMysis("test", "ollama", "llama3", 0.5)
    // ... verify temperature is stored and retrieved
}

// commander_test.go
func TestCommanderConfigureMysisWithModel(t *testing.T) {
    // ... verify model can be changed per-mysis
}
```

---

## Phase 9: Verification

1. Run `make test` - all tests must pass
2. Run `make build` - compilation must succeed
3. Manual test: create mysis, configure with different model, verify provider uses correct model
4. Update `KNOWN_ISSUES.md` to mark first item complete

---

## Summary of Files Changed

| File | Action |
|------|--------|
| `documentation/OPENCODE_ZEN_MODELS.md` | Create |
| `internal/config/config.go` | Modify |
| `internal/config/config_test.go` | Modify |
| `config.toml` | Modify |
| `internal/store/schema.sql` | Modify |
| `internal/store/myses.go` | Modify |
| `internal/store/store_test.go` | Modify |
| `internal/provider/provider.go` | Modify |
| `internal/provider/factory.go` | Create |
| `internal/provider/ollama.go` | Modify |
| `internal/provider/opencode.go` | Modify |
| `internal/provider/mock.go` | Modify |
| `internal/core/commander.go` | Modify |
| `internal/core/commander_test.go` | Modify |
| `internal/tui/input.go` | Modify |
| `internal/tui/app.go` | Modify |
| `cmd/zoea/main.go` | Modify |
| `documentation/KNOWN_ISSUES.md` | Modify |

---

## Database Migration Note

This change requires schema v4 with a new `temperature` column. Per AGENTS.md, no data migrations are supported. Reset with:

```bash
make db-reset-accounts
```

This exports usernames/passwords to `accounts-backup.sql` before recreating the DB.

---

## Implementation Order

1. Documentation (OpenCode Zen models)
2. Config layer (add temperature field)
3. Store layer (schema + CRUD)
4. Provider layer (factory pattern)
5. Commander layer (use factories)
6. TUI layer (model selection)
7. Main.go (register factories)
8. Tests (verify all layers)
9. Verification (build + manual test)
