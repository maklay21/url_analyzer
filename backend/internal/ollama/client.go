package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

func New(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) GenerateSummary(ctx context.Context, text string) (string, error) {
	// Ограничиваем длину контекста (Ollama не бесконечный)
	const maxChars = 12000
	if len(text) > maxChars {
		text = text[:maxChars]
	}

	prompt := fmt.Sprintf(`Проанализируй следующий текст и определи, о чём этот сайт. 
Если информации недостаточно для содержательного анализа, ответь строго: "Недостаточно данных для анализа".
В противном случае дай краткое резюме (2-3 предложения) о тематике сайта.

Текст:
%s

Ответ:`, text)

	reqBody := GenerateRequest{
		Model:  c.model,
		Prompt: prompt,
		System: "Ты — аналитик контента. Отвечай на русском языке.",
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode ollama response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("ollama error: %s", result.Error)
	}

	return result.Response, nil
}
