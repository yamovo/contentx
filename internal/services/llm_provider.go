package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openaiLLMProvider calls an OpenAI-compatible /chat/completions endpoint.
// Works with OpenAI, DeepSeek, Ollama, vLLM, LocalAI, and any other server
// that mimics the OpenAI Chat Completion API.
type openaiLLMProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewOpenAILLMProvider creates an LLM provider that calls an OpenAI-compatible
// chat completions API. If baseURL is empty it defaults to "https://api.openai.com/v1".
func NewOpenAILLMProvider(apiKey, baseURL, model string) LLMProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &openaiLLMProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (o *openaiLLMProvider) Name() string { return "openai" }
func (o *openaiLLMProvider) External() bool {
	return true
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func (o *openaiLLMProvider) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: o.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat API call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat API returned %d: %s", resp.StatusCode, string(raw))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("chat API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("chat API returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

// NewLLMProvider is the factory used by routes.go to construct the LLM provider
// from configuration. It never returns nil: when no valid provider is configured
// it falls back to dummy so the system still boots.
func NewLLMProvider(provider, apiKey, baseURL, model string) LLMProvider {
	switch provider {
	case "openai", "deepseek", "ollama", "localai", "vllm":
		if apiKey == "" && provider == "openai" {
			break
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
		return NewOpenAILLMProvider(apiKey, baseURL, model)
	}
	return NewDummyLLM()
}
