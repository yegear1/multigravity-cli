package gateway

import (
	"encoding/json"
	"strings"
)

// OpenAI Chat Completion Request
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Profile     string        `json:"profile,omitempty"`
}

// ChatMessage represents a single message in an OpenAI conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // can be string or []ChatMessagePart
	Name    string `json:"name,omitempty"`
}

// ChatMessagePart represents a multimodal part in an OpenAI message
type ChatMessagePart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *ImageURLParam `json:"image_url,omitempty"`
}

type ImageURLParam struct {
	URL string `json:"url"`
}

// ContentString extracts the plain text content from ChatMessage
func (m ChatMessage) ContentString() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []any:
		var sb strings.Builder
		for _, item := range v {
			if mItem, ok := item.(map[string]any); ok {
				if mItem["type"] == "text" {
					if t, ok := mItem["text"].(string); ok {
						if sb.Len() > 0 {
							sb.WriteString("\n")
						}
						sb.WriteString(t)
					}
				}
			}
		}
		return sb.String()
	case []ChatMessagePart:
		var sb strings.Builder
		for _, part := range v {
			if part.Type == "text" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(part.Text)
			}
		}
		return sb.String()
	default:
		// Attempt JSON unmarshal if raw json
		if raw, ok := v.(json.RawMessage); ok {
			var str string
			if err := json.Unmarshal(raw, &str); err == nil {
				return str
			}
			var parts []ChatMessagePart
			if err := json.Unmarshal(raw, &parts); err == nil {
				var sb strings.Builder
				for _, part := range parts {
					if part.Type == "text" {
						if sb.Len() > 0 {
							sb.WriteString("\n")
						}
						sb.WriteString(part.Text)
					}
				}
				return sb.String()
			}
		}
		return ""
	}
}

// OpenAI Chat Completion Response (Non-streaming)
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   UsageInfo              `json:"usage"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAI Chat Completion Chunk (Streaming)
type ChatCompletionChunk struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
}

type ChatCompletionChunkChoice struct {
	Index        int              `json:"index"`
	Delta        ChatMessageDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

type ChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// OpenAI Models catalog
type ModelListResponse struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAI Error format
type OpenAIErrorResponse struct {
	Error OpenAIErrorDetail `json:"error"`
}

type OpenAIErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// CloudCode upstream request types
type CloudCodeRequest struct {
	Model   string               `json:"model"`
	Project string               `json:"project,omitempty"`
	Request CloudCodeRequestBody `json:"request"`
}

type CloudCodeRequestBody struct {
	Model             string                      `json:"model"`
	Contents          []CloudCodeContent          `json:"contents"`
	SystemInstruction *CloudCodeSystemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *CloudCodeGenerationConfig  `json:"generationConfig,omitempty"`
}

type CloudCodeContent struct {
	Role  string          `json:"role"` // "user" or "model"
	Parts []CloudCodePart `json:"parts"`
}

type CloudCodePart struct {
	Text       string               `json:"text,omitempty"`
	InlineData *CloudCodeInlineData `json:"inlineData,omitempty"`
}

type CloudCodeInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

type CloudCodeSystemInstruction struct {
	Parts []CloudCodePart `json:"parts"`
}

type CloudCodeGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type CloudCodeImagePart struct {
	MimeType string
	Data     string
}
