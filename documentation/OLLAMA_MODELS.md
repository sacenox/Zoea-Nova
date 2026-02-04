# Ollama Models for Zoea Nova

Hardware: **NVIDIA GeForce RTX 3080 Ti (12GB VRAM)**

This document catalogs Ollama models suitable for our setup, focusing on tool calling support (required for MCP integration) and thinking/reasoning capabilities.

---

## TL;DR Recommendations

| Use Case | Model | Size | Pull Command |
|----------|-------|------|--------------|
| **Fast iteration/testing** | Qwen 3 4B | 2.6GB | `ollama pull qwen3:4b` |
| **Balanced (recommended)** | Qwen 3 8B | 4.9GB | `ollama pull qwen3:8b` |
| **Best quality** | Qwen 3 14B | 9.0GB | `ollama pull qwen3:14b` |
| **Reasoning specialist** | DeepSeek R1 Tool Calling 14B | 9.0GB | `ollama pull MFDoom/deepseek-r1-tool-calling:14b` |

---

## Smallest Models with Tool Support

### Llama 3.2 (Meta)

| Variant | Size | Context | Tool Support | Notes |
|---------|------|---------|--------------|-------|
| 1B | 1.3GB | 128K | Limited | Edge/mobile optimized |
| **3B** | 2.0GB | 128K | ✅ Yes | Outperforms Gemma 2 2.6B on tool use |

```bash
ollama pull llama3.2:3b
```

**Pros:** Extremely lightweight, fast inference, good for simple tool chains.
**Cons:** Limited reasoning depth, may struggle with complex multi-step tool sequences.

---

### Qwen 3 (Alibaba) ⭐ Recommended

Best balance of size, tool calling, and thinking capabilities.

| Variant | Size | Context | Tool Support | Thinking | Notes |
|---------|------|---------|--------------|----------|-------|
| 0.6B | 0.4GB | 32K | ✅ Yes | ✅ Yes | Minimal, edge only |
| 1.7B | 1.1GB | 32K | ✅ Yes | ✅ Yes | Lightweight |
| **4B** | 2.6GB | 32K | ✅ Yes | ✅ Yes | Fast iteration |
| **8B** | 4.9GB | 32K | ✅ Yes | ✅ Yes | Sweet spot |
| **14B** | 9.0GB | 32K | ✅ Yes | ✅ Yes | Best quality for 12GB |
| 30B MoE | 18GB | 32K | ✅ Yes | ✅ Yes | Won't fit in VRAM |

```bash
# Recommended for development
ollama pull qwen3:4b

# Recommended for production
ollama pull qwen3:8b

# Best quality (fits in 12GB)
ollama pull qwen3:14b
```

**Thinking mode control:**
- Enable: append `/think` to prompt
- Disable: append `/nothink` to prompt
- Set default in system prompt

**Pros:** Native tool calling + thinking in same model, excellent structured output, streaming tool calls.
**Cons:** Thinking mode can be verbose (no token limit control yet).

---

### Qwen 2.5

Predecessor to Qwen 3, still solid for tool calling without thinking features.

| Variant | Size | Context | Tool Support | Notes |
|---------|------|---------|--------------|-------|
| 7B | 4.7GB | 32K | ✅ Yes | Good JSON output |
| 14B | 9.0GB | 32K | ✅ Yes | Excellent structured data |

```bash
ollama pull qwen2.5:7b
ollama pull qwen2.5:14b
```

---

### Mistral 7B v0.3

| Variant | Size | Context | Tool Support | Notes |
|---------|------|---------|--------------|-------|
| 7B | 4.1GB | 32K | ✅ Yes | Uses raw mode |

```bash
ollama pull mistral:7b
```

**Pros:** Well-established, good general performance.
**Cons:** Tool calling uses older raw mode format, no native thinking.

---

## Thinking/Reasoning Models with Tool Support

### LFM2.5-Thinking (Liquid AI)

Ultra-compact reasoning model. 1.2B params but matches Qwen3-1.7B on benchmarks.

| Variant | Size | Context | Tool Support | Thinking | Notes |
|---------|------|---------|--------------|----------|-------|
| **1.2B** (Q4_K_M) | 731MB | 32K | ⚠️ No native | ✅ Yes | On-device capable |
| 1.2B (Q8_0) | 1.2GB | 32K | ⚠️ No native | ✅ Yes | Higher precision |

```bash
ollama pull lfm2.5-thinking
```

**⚠️ Tool Calling Limitation:**
LFM2.5-Thinking scores 56.97 on BFCLv3 (Berkeley Function Calling Leaderboard), meaning it *understands* function calling concepts. However, it does **NOT support Ollama's native `tools` API parameter**. Tool definitions must be embedded in the prompt and output parsed manually.

**Not recommended for Zoea Nova** — use Qwen 3 instead for native tool support.

**Benchmarks vs non-thinking variant:**
- Math (MATH-500): 63 → **88**
- Instruction following: 61 → **69**

**Pros:** Extremely small (fits in 731MB), thinking traces built-in.
**Cons:** No native Ollama tool API support. Requires manual prompt engineering for tools.

---

### Qwen 3 (see above)

All Qwen 3 variants support both thinking AND tool calling natively. This is the recommended choice.

---

### DeepSeek R1 Distill (with Tool Calling)

Community-maintained models that add tool calling to DeepSeek's reasoning models.

| Variant | Size | Quantization | Tool Support | Notes |
|---------|------|--------------|--------------|-------|
| 7B Qwen | 4.7GB | Q4_K_M | ✅ Yes | Qwen architecture |
| **8B Llama** | 4.9GB | Q4_K_M | ✅ Yes | Llama architecture |
| **14B Qwen** | 9.0GB | Q4_K_M | ✅ Yes | Best reasoning |

```bash
# 8B variant
ollama pull MFDoom/deepseek-r1-tool-calling:8b

# 14B variant (recommended for reasoning tasks)
ollama pull MFDoom/deepseek-r1-tool-calling:14b
```

**Pros:** Strong reasoning comparable to OpenAI o1, MIT licensed.
**Cons:** Community-maintained (not official), tool calling added via custom templates.

---

## VRAM Usage Guidelines

With 12GB VRAM and Q4_K_M quantization:

| Model Size | VRAM Usage | Performance | Recommendation |
|------------|------------|-------------|----------------|
| ≤4B | ~3GB | 80+ tok/s | ✅ Fast iteration |
| 7-8B | ~5GB | 50-60 tok/s | ✅ Balanced |
| 14B | ~9GB | 40+ tok/s | ✅ Best quality |
| 30B+ | >12GB | Won't fit | ❌ Avoid |

**Note:** Context length affects VRAM. Long conversations may require smaller models or context pruning.

---

## Configuration for Zoea Nova

Suggested `config.toml` provider settings:

```toml
[providers.ollama]
endpoint = "http://localhost:11434"

# For fast testing
model = "qwen3:4b"

# For production Myses
# model = "qwen3:8b"

# For complex reasoning tasks
# model = "qwen3:14b"
```

### Thinking Mode Strategy

For game Myses that need to plan:
1. Use Qwen 3 with thinking enabled for strategic decisions
2. Disable thinking (`/nothink`) for rapid tool execution
3. Consider separate models: fast (4B) for actions, larger (14B) for planning

---

## Quick Start

```bash
# Install recommended models
ollama pull qwen3:4b    # 2.6GB - fast testing
ollama pull qwen3:8b    # 4.9GB - balanced

# Verify tool support
ollama run qwen3:8b
>>> /show info
```

Test tool calling:
```bash
curl http://localhost:11434/api/chat -d '{
  "model": "qwen3:8b",
  "messages": [{"role": "user", "content": "What is 2+2?"}],
  "tools": [{
    "type": "function",
    "function": {
      "name": "calculate",
      "description": "Perform arithmetic",
      "parameters": {
        "type": "object",
        "properties": {
          "expression": {"type": "string"}
        },
        "required": ["expression"]
      }
    }
  }]
}'
```

---

## Currently Installed

```
llama3.1:8b           4.9 GB    ✅ native tool support
qwen3:4b              2.5 GB    ✅ recommended (default)
qwen3:8b              5.2 GB    ✅ recommended
lfm2.5-thinking       731 MB    ⚠️ no native tool API
```

---

## References

- [Ollama Tool Calling Docs](https://docs.ollama.com/capabilities/tool-calling)
- [Ollama Thinking Docs](https://docs.ollama.com/capabilities/thinking)
- [Qwen 3 on Ollama](https://ollama.com/library/qwen3)
- [LFM2.5-Thinking on Ollama](https://ollama.com/library/lfm2.5-thinking)
- [LFM2.5-Thinking Blog Post](https://www.liquid.ai/blog/lfm2-5-1-2b-thinking-on-device-reasoning-under-1gb)
- [DeepSeek R1 Tool Calling](https://ollama.com/MFDoom/deepseek-r1-tool-calling)
