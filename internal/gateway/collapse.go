package gateway

import (
	"encoding/json"
	"regexp"
	"strings"
)

var dataURIRegex = regexp.MustCompile(`^data:(image\/[^;]+);base64,(.+)$`)

// ExtractPartsAndImages extracts text and any inline base64 images from a ChatMessage
func ExtractPartsAndImages(msg ChatMessage) (string, []CloudCodeImagePart) {
	var textParts []string
	var images []CloudCodeImagePart

	switch v := msg.Content.(type) {
	case string:
		textParts = append(textParts, strings.TrimSpace(v))
	case []any:
		for _, rawItem := range v {
			if mItem, ok := rawItem.(map[string]any); ok {
				pType, _ := mItem["type"].(string)
				switch pType {
				case "text":
					if t, ok := mItem["text"].(string); ok && strings.TrimSpace(t) != "" {
						textParts = append(textParts, strings.TrimSpace(t))
					}
				case "image_url":
					if iu, ok := mItem["image_url"].(map[string]any); ok {
						if url, ok := iu["url"].(string); ok {
							if match := dataURIRegex.FindStringSubmatch(url); len(match) == 3 {
								images = append(images, CloudCodeImagePart{
									MimeType: match[1],
									Data:     match[2],
								})
							}
						}
					}
				}
			}
		}
	case []ChatMessagePart:
		for _, part := range v {
			switch part.Type {
			case "text":
				if strings.TrimSpace(part.Text) != "" {
					textParts = append(textParts, strings.TrimSpace(part.Text))
				}
			case "image_url":
				if part.ImageURL != nil {
					if match := dataURIRegex.FindStringSubmatch(part.ImageURL.URL); len(match) == 3 {
						images = append(images, CloudCodeImagePart{
							MimeType: match[1],
							Data:     match[2],
						})
					}
				}
			}
		}
	default:
		// Attempt unmarshaling raw json
		if raw, ok := v.(json.RawMessage); ok {
			var str string
			if err := json.Unmarshal(raw, &str); err == nil {
				textParts = append(textParts, strings.TrimSpace(str))
			} else {
				var parts []ChatMessagePart
				if err := json.Unmarshal(raw, &parts); err == nil {
					for _, p := range parts {
						if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
							textParts = append(textParts, strings.TrimSpace(p.Text))
						} else if p.Type == "image_url" && p.ImageURL != nil {
							if match := dataURIRegex.FindStringSubmatch(p.ImageURL.URL); len(match) == 3 {
								images = append(images, CloudCodeImagePart{
									MimeType: match[1],
									Data:     match[2],
								})
							}
						}
					}
				}
			}
		}
	}

	return strings.Join(textParts, "\n"), images
}

// CollapseOpenAIMessages collapses an OpenAI message array into CloudCode format
// It extracts system messages into systemPrompt, maps assistant roles to "model",
// and packages inline base64 images into CloudCode parts.
func CollapseOpenAIMessages(messages []ChatMessage) ([]CloudCodeContent, string) {
	var contents []CloudCodeContent
	var systemBlocks []string

	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		text, images := ExtractPartsAndImages(msg)

		if text == "" && len(images) == 0 {
			continue
		}

		if role == "system" {
			if text != "" {
				systemBlocks = append(systemBlocks, text)
			}
			continue
		}

		// CloudCode expects role: "user" or "model"
		cloudRole := "user"
		if role == "assistant" || role == "model" {
			cloudRole = "model"
		}

		var parts []CloudCodePart
		// Images first so visual context is established before text instructions
		for _, img := range images {
			parts = append(parts, CloudCodePart{
				InlineData: &CloudCodeInlineData{
					MimeType: img.MimeType,
					Data:     img.Data,
				},
			})
		}
		if text != "" {
			parts = append(parts, CloudCodePart{
				Text: text,
			})
		}

		contents = append(contents, CloudCodeContent{
			Role:  cloudRole,
			Parts: parts,
		})
	}

	systemPrompt := strings.Join(systemBlocks, "\n\n")
	return contents, systemPrompt
}
