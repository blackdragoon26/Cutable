package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenRouter struct {
	apiKey string
	model  string
	client *http.Client
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type CompletionRequest struct {
	Model             string           `json:"model"`
	Messages          []Message        `json:"messages"`
	Tools             []ToolDefinition `json:"tools,omitempty"`
	Temperature       float64          `json:"temperature,omitempty"`
	ParallelToolCalls bool             `json:"parallel_tool_calls"`
}

type CompletionResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

func NewOpenRouter(apiKey, model string) *OpenRouter {
	return &OpenRouter{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 2 * time.Minute},
	}
}

func (o *OpenRouter) Complete(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	payload, err := json.Marshal(CompletionRequest{
		Model:             o.model,
		Messages:          messages,
		Tools:             tools,
		Temperature:       0.2,
		ParallelToolCalls: false,
	})
	if err != nil {
		return Message{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/blackdragoon26/Cutable")
	req.Header.Set("X-Title", "Cutable")

	res, err := o.client.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("openrouter request: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return Message{}, err
	}
	var result CompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return Message{}, fmt.Errorf("decode openrouter response (status %d): %w", res.StatusCode, err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if result.Error != nil {
			return Message{}, fmt.Errorf("openrouter status %d: %s", res.StatusCode, result.Error.Message)
		}
		return Message{}, fmt.Errorf("openrouter status %d", res.StatusCode)
	}
	if len(result.Choices) == 0 {
		return Message{}, fmt.Errorf("openrouter returned no choices")
	}
	return result.Choices[0].Message, nil
}
