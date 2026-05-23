package service

var modelMaxContextTokens = map[string]int{
	"openai/gpt-4o":                    128000,
	"openai/gpt-4o-mini":               128000,
	"anthropic/claude-sonnet-4-6":      200000,
	"anthropic/claude-haiku-4-5":       200000,
	"gemini/gemini-2.5-flash":          1000000,
	"ollama/llama3.2":                  128000,
	"ollama/llama3.1":                  128000,
	"ollama/mistral":                   32000,
	"ollama/phi4":                      128000,
	"deepseek/deepseek-chat":           65536,
	"deepseek/deepseek-reasoner":       65536,
}

const defaultContextWindow = 32000

func contextWindow(provider, model string) int {
	key := provider + "/" + model
	if n, ok := modelMaxContextTokens[key]; ok {
		return n
	}
	return defaultContextWindow
}

func estimateTokens(s string) int {
	n := len(s) / 4
	if n < 1 {
		return 1
	}
	return n
}
