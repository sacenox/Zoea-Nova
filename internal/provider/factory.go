package provider

import "golang.org/x/time/rate"

type OllamaFactory struct {
	name     string
	endpoint string
	limiter  *rate.Limiter
}

func NewOllamaFactory(name string, endpoint string, rateLimit float64, rateBurst int) *OllamaFactory {
	return &OllamaFactory{
		name:     name,
		endpoint: endpoint,
		limiter:  rate.NewLimiter(rate.Limit(rateLimit), rateBurst),
	}
}

func (f *OllamaFactory) Name() string { return f.name }

func (f *OllamaFactory) Create(model string, temperature float64) Provider {
	return NewOllamaWithTemp(f.name, f.endpoint, model, temperature, f.limiter)
}

type OpenCodeFactory struct {
	name     string
	endpoint string
	apiKey   string
	limiter  *rate.Limiter
}

func NewOpenCodeFactory(name string, endpoint, apiKey string, rateLimit float64, rateBurst int) *OpenCodeFactory {
	return &OpenCodeFactory{
		name:     name,
		endpoint: endpoint,
		apiKey:   apiKey,
		limiter:  rate.NewLimiter(rate.Limit(rateLimit), rateBurst),
	}
}

func (f *OpenCodeFactory) Name() string { return f.name }

func (f *OpenCodeFactory) Create(model string, temperature float64) Provider {
	return NewOpenCodeWithTemp(f.name, f.endpoint, model, f.apiKey, temperature, f.limiter)
}
