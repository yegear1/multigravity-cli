package gateway

// AnthropicMessageRequest represents the incoming payload for POST /v1/messages
type AnthropicMessageRequest struct {
	Model         string             `json:"model"`
	Messages      []AnthropicMessage `json:"messages"`
	System        any                `json:"system,omitempty"` // string or []AnthropicContentBlock
	MaxTokens     int                `json:"max_tokens"`
	Temperature   *float64           `json:"temperature,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Profile       string             `json:"profile,omitempty"`
	Strategy      string             `json:"strategy,omitempty"`
	Failover      *bool              `json:"failover,omitempty"`
}

// AnthropicMessage represents a turn in an Anthropic conversation
type AnthropicMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content any    `json:"content"` // string or []AnthropicContentBlock
}

// AnthropicContentBlock represents a content element (text or image)
type AnthropicContentBlock struct {
	Type   string                `json:"type"` // "text" or "image"
	Text   string                `json:"text,omitempty"`
	Source *AnthropicImageSource `json:"source,omitempty"`
}

// AnthropicImageSource represents the base64 source for images
type AnthropicImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // e.g. "image/jpeg", "image/png"
	Data      string `json:"data"`       // raw base64 string
}

// AnthropicUsage tracks token consumption
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicMessageResponse represents a non-streaming response for POST /v1/messages
type AnthropicMessageResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"` // "message"
	Role         string                  `json:"role"` // "assistant"
	Content      []AnthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"` // "end_turn", "max_tokens", "stop_sequence"
	StopSequence *string                 `json:"stop_sequence"`
	Usage        AnthropicUsage          `json:"usage"`
}

// Anthropic Streaming Event Payloads

type AnthropicMessageStartEvent struct {
	Type    string                   `json:"type"` // "message_start"
	Message AnthropicMessageResponse `json:"message"`
}

type AnthropicContentBlockStartEvent struct {
	Type         string                `json:"type"` // "content_block_start"
	Index        int                   `json:"index"`
	ContentBlock AnthropicContentBlock `json:"content_block"`
}

type AnthropicContentBlockDeltaEvent struct {
	Type  string             `json:"type"` // "content_block_delta"
	Index int                `json:"index"`
	Delta AnthropicTextDelta `json:"delta"`
}

type AnthropicTextDelta struct {
	Type string `json:"type"` // "text_delta"
	Text string `json:"text"`
}

type AnthropicContentBlockStopEvent struct {
	Type  string `json:"type"` // "content_block_stop"
	Index int    `json:"index"`
}

type AnthropicMessageDeltaEvent struct {
	Type  string                `json:"type"` // "message_delta"
	Delta AnthropicMessageDelta `json:"delta"`
	Usage AnthropicUsage        `json:"usage"`
}

type AnthropicMessageDelta struct {
	StopReason   string  `json:"stop_reason"`
	StopSequence *string `json:"stop_sequence"`
}

type AnthropicMessageStopEvent struct {
	Type string `json:"type"` // "message_stop"
}

// Anthropic Error format
type AnthropicErrorResponse struct {
	Type  string               `json:"type"` // "error"
	Error AnthropicErrorDetail `json:"error"`
}

type AnthropicErrorDetail struct {
	Type    string `json:"type"` // "invalid_request_error", "rate_limit_error", "api_error", "authentication_error"
	Message string `json:"message"`
}
