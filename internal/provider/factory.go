package provider

import "golang.org/x/time/rate"

type OllamaFactory struct {
	endpoint string
	limiter  *rate.Limiter
}

func NewOllamaFactory(endpoint string, rateLimit float64, rateBurst int) *OllamaFactory {
	return &OllamaFactory{
		endpoint: endpoint,
		limiter:  rate.NewLimiter(rate.Limit(rateLimit), rateBurst),
	}
}

func (f *OllamaFactory) Name() string { return "ollama" }

func (f *OllamaFactory) Create(model string, temperature float64) Provider {
	return NewOllamaWithTemp(f.endpoint, model, temperature, f.limiter)
}

type OpenCodeFactory struct {
	endpoint string
	apiKey   string
	limiter  *rate.Limiter
}

func NewOpenCodeFactory(endpoint, apiKey string, rateLimit float64, rateBurst int) *OpenCodeFactory {
	return &OpenCodeFactory{
		endpoint: endpoint,
		apiKey:   apiKey,
		limiter:  rate.NewLimiter(rate.Limit(rateLimit), rateBurst),
	}
}

func (f *OpenCodeFactory) Name() string { return "opencode_zen" }

func (f *OpenCodeFactory) Create(model string, temperature float64) Provider {
	return NewOpenCodeWithTemp(f.endpoint, model, f.apiKey, temperature, f.limiter)
}
