# Hardcoded Provider/Model Configuration Audit

**Date:** 2026-02-07  
**Scope:** Non-test, non-provider implementation files  
**Goal:** Identify all hardcoded provider/model references and determine if they should be dynamic

---

## Executive Summary

**Total Issues Found:** 3 (1 high-priority, 2 acceptable)

### High-Priority Issues
1. **API Key Lookup Hardcoded** - `cmd/zoea/main.go:244`

### Acceptable Hardcoding
1. **Environment Variable Names** - `internal/config/config.go:173-222`
2. **Provider Type Identifiers** - `internal/provider/*.go` (factory names, logging)

---

## Detailed Findings

### 1. API Key Lookup Hardcoded ⚠️ HIGH PRIORITY

**File:** `cmd/zoea/main.go:244`

```go
apiKey := creds.GetAPIKey("opencode_zen")
```

**Context:**  
Provider initialization logic detects OpenCode providers by endpoint URL pattern (`opencode.ai`), then uses a single hardcoded credential key `"opencode_zen"` for ALL OpenCode providers.

**Problem:**  
- Config supports multiple OpenCode provider configs (`zen-nano`, `zen-pickle`) but all share ONE API key
- No way to use different API keys for different OpenCode providers
- Breaks provider isolation principle

**Current Detection Logic:**
```go
for name, provCfg := range cfg.Providers {
    if strings.Contains(provCfg.Endpoint, "localhost:11434") || strings.Contains(provCfg.Endpoint, "/ollama") {
        factory := provider.NewOllamaFactory(provCfg.Endpoint, provCfg.RateLimit, provCfg.RateBurst)
        registry.RegisterFactory(name, factory)
    } else if strings.Contains(provCfg.Endpoint, "opencode.ai") {
        apiKey := creds.GetAPIKey("opencode_zen")  // ← HARDCODED
        if apiKey != "" {
            factory := provider.NewOpenCodeFactory(provCfg.Endpoint, apiKey, provCfg.RateLimit, provCfg.RateBurst)
            registry.RegisterFactory(name, factory)
        }
    }
}
```

**Recommended Fix:**  
Use provider config name as credential key:

```go
for name, provCfg := range cfg.Providers {
    if strings.Contains(provCfg.Endpoint, "localhost:11434") || strings.Contains(provCfg.Endpoint, "/ollama") {
        factory := provider.NewOllamaFactory(provCfg.Endpoint, provCfg.RateLimit, provCfg.RateBurst)
        registry.RegisterFactory(name, factory)
    } else if strings.Contains(provCfg.Endpoint, "opencode.ai") {
        // Try provider-specific key first, fallback to "opencode_zen" for backward compatibility
        apiKey := creds.GetAPIKey(name)
        if apiKey == "" {
            apiKey = creds.GetAPIKey("opencode_zen")
        }
        if apiKey != "" {
            factory := provider.NewOpenCodeFactory(provCfg.Endpoint, apiKey, provCfg.RateLimit, provCfg.RateBurst)
            registry.RegisterFactory(name, factory)
        }
    }
}
```

**Migration Path:**
1. Keep `"opencode_zen"` as fallback for existing users
2. Document new per-provider credential keys in README
3. Example usage:
   ```bash
   # Old way (still works)
   export ZOEA_OPENCODE_API_KEY="key123"
   
   # New way (per-provider keys)
   export ZOEA_ZEN_NANO_API_KEY="key123"
   export ZOEA_ZEN_PICKLE_API_KEY="key456"
   ```

---

### 2. Environment Variable Names ✅ ACCEPTABLE

**File:** `internal/config/config.go:173-222`

**Hardcoded Values:**
```go
{"ZOEA_OLLAMA_ENDPOINT", func(v string) {
    updateProvider("ollama", func(p ProviderConfig) ProviderConfig { p.Endpoint = v; return p })
}}
{"ZOEA_OLLAMA_MODEL", func(v string) { ... }}
{"ZOEA_OLLAMA_TEMPERATURE", func(v string) { ... }}
// ... etc for opencode_zen
```

**Context:**  
Environment variable overrides for specific provider configs (`ollama`, `opencode_zen`).

**Why Acceptable:**
- Environment variables MUST have stable names (external contract)
- These are convenience overrides for default providers
- Users can still define custom providers in `config.toml` without env vars
- Changing these would break existing user scripts

**Recommendation:**  
Keep as-is. Document in README that these env vars apply to the `[providers.ollama]` and `[providers.opencode_zen]` config sections specifically.

---

### 3. Provider Type Identifiers ✅ ACCEPTABLE

**Files:**
- `internal/provider/ollama.go:54` - `return "ollama"`
- `internal/provider/opencode.go:82` - `return "opencode_zen"`
- `internal/provider/factory.go:17` - `func (f *OllamaFactory) Name() string { return "ollama" }`
- `internal/provider/factory.go:38` - `func (f *OpenCodeFactory) Name() string { return "opencode_zen" }`

**Context:**  
Provider implementation returns its type identifier for logging and debugging.

**Why Acceptable:**
- These are **internal type identifiers**, not config keys
- Used only for logging (`Str("provider", "ollama")`) and error messages
- Factory `Name()` returns implementation type, not config key
- Config key (e.g., `"zen-nano"`) is stored separately in registry

**Example:**
```go
// Config key: "zen-nano"
// Factory type: "opencode_zen" (implementation detail)
registry.RegisterFactory("zen-nano", factory)
```

**Recommendation:**  
Keep as-is. These are implementation details, not user-facing config.

---

## Model Name References

**Searched for:** `qwen3`, `llama3`, `gpt-5-nano`, `big-pickle`

**Findings:**  
- **Zero references** outside of:
  - Test files (`*_test.go`)
  - Config file (`config.toml`)
  - Documentation

**Conclusion:**  
Model names are already fully dynamic. All model references come from config or user input. No hardcoding found.

---

## Provider Detection Logic

**File:** `cmd/zoea/main.go:238-242`

**Current Implementation:**
```go
if strings.Contains(provCfg.Endpoint, "localhost:11434") || strings.Contains(provCfg.Endpoint, "/ollama") {
    // Ollama
} else if strings.Contains(provCfg.Endpoint, "opencode.ai") {
    // OpenCode
}
```

**Analysis:**
- Uses URL pattern matching to determine provider type
- Fragile: breaks if Ollama runs on different port or OpenCode changes domain
- No explicit provider type in config

**Alternative Approach:**
Add explicit `type` field to `ProviderConfig`:

```toml
[providers.ollama]
type = "ollama"
endpoint = "http://localhost:11434"
model = "qwen3:8b"

[providers.zen-nano]
type = "opencode"
endpoint = "https://opencode.ai/v1"
model = "gpt-5-nano"
```

**Pros:**
- Explicit, self-documenting
- Works with any endpoint URL
- Easier to add new provider types

**Cons:**
- Requires config file migration
- Breaking change for existing users

**Recommendation:**  
Consider for v1.0 breaking changes. For now, URL pattern matching works for known use cases.

---

## Summary of Recommendations

### Immediate Action Required
1. **Fix API key lookup** - Use provider config name as credential key with fallback to `"opencode_zen"`

### Future Improvements
1. **Add explicit provider type field** - Consider for v1.0 (breaking change)
2. **Document env var behavior** - Clarify that `ZOEA_OLLAMA_*` applies to `[providers.ollama]` config section

### No Action Needed
1. Environment variable names (external contract)
2. Provider type identifiers (internal implementation detail)
3. Model names (already fully dynamic)

---

## Verification

To verify no other hardcoded references:

```bash
# Search for provider/model string literals
rg '"ollama"|"opencode_zen"|"qwen3"|"llama3"|"gpt-5-nano"|"big-pickle"' \
  --type go \
  --glob '!*_test.go' \
  --glob '!internal/provider/*.go' \
  --glob '!documentation/**'

# Should only return:
# - cmd/zoea/main.go:244 (API key lookup - needs fix)
# - internal/config/config.go:173-222 (env vars - acceptable)
# - internal/provider/factory.go:17,38 (type identifiers - acceptable)
```

---

**End of Report**
