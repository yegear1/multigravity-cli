package gateway

import (
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/quota"
)

func cloudRequestText(req *CloudCodeRequest) string {
	if req == nil {
		return ""
	}
	var b strings.Builder
	if req.Request.SystemInstruction != nil {
		for _, part := range req.Request.SystemInstruction.Parts {
			b.WriteString(part.Text)
		}
	}
	for _, content := range req.Request.Contents {
		for _, part := range content.Parts {
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

func resolveUsage(upstream UpstreamUsage, promptText, completionText string) (UsageInfo, bool) {
	if upstream.Observed && (upstream.TotalTokens > 0 || upstream.PromptTokens > 0 || upstream.CompletionTokens > 0) {
		total := upstream.TotalTokens
		if total == 0 {
			total = upstream.PromptTokens + upstream.CompletionTokens
		}
		return UsageInfo{
			PromptTokens:     upstream.PromptTokens,
			CompletionTokens: upstream.CompletionTokens,
			TotalTokens:      total,
		}, false
	}
	prompt := quota.EstimateTokens(promptText)
	completion := quota.EstimateTokens(completionText)
	return UsageInfo{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}, true
}

func recordGatewayUsage(profileName, model string, upstream UpstreamUsage, promptText, completionText string) UsageInfo {
	info, estimated := resolveUsage(upstream, promptText, completionText)
	if info.TotalTokens > 0 {
		_ = quota.RecordTokens(profileName, quota.SourceGateway, model, info.PromptTokens, info.CompletionTokens, info.TotalTokens, estimated)
	}
	return info
}
