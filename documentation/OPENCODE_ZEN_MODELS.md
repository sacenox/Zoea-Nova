# OpenCode Zen Models

Models available through OpenCode Zen for use with Zoea Nova.

## Free Models

These models are free during their beta period:

| Model | Model ID | Endpoint | Tool Support |
| --- | --- | --- | --- |
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
