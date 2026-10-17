package gateway

import (
	"net/http"
	"os"
	"strings"

	"github.com/ye-dev/multigravity-cli/internal/quota"
	"github.com/ye-dev/multigravity-cli/internal/telemetry"
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

func extractTraceContext(r *http.Request) (string, string) {
	if r == nil {
		traceID := strings.TrimSpace(os.Getenv("MAESTRI_TRACE_ID"))
		if traceID == "" {
			traceID = strings.TrimSpace(os.Getenv("MULTIGRAVITY_TRACE_ID"))
		}
		return traceID, ""
	}
	traceID := strings.TrimSpace(r.Header.Get("X-Trace-ID"))
	if traceID == "" {
		traceID = strings.TrimSpace(r.Header.Get("X-Maestri-Trace-ID"))
	}
	if traceID == "" {
		traceID = strings.TrimSpace(r.Header.Get("Traceparent"))
	}
	if traceID == "" {
		traceID = strings.TrimSpace(r.URL.Query().Get("trace_id"))
	}
	if traceID == "" {
		traceID = strings.TrimSpace(os.Getenv("MAESTRI_TRACE_ID"))
	}
	if traceID == "" {
		traceID = strings.TrimSpace(os.Getenv("MULTIGRAVITY_TRACE_ID"))
	}

	taskID := strings.TrimSpace(r.Header.Get("X-Task-ID"))
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("task_id"))
	}
	if taskID == "" {
		taskID = strings.TrimSpace(os.Getenv("MULTIGRAVITY_TASK_ID"))
	}
	return traceID, taskID
}

func resolveUsage(model string, upstream UpstreamUsage, promptText, completionText string) (UsageInfo, bool) {
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
	prompt := quota.EstimateSegmentTokens(model, "prompt", promptText)
	completion := quota.EstimateSegmentTokens(model, "completion", completionText)
	return UsageInfo{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}, true
}

func recordGatewayUsage(profileName, model string, upstream UpstreamUsage, promptText, completionText string) UsageInfo {
	return recordGatewayUsageWithTrace(profileName, model, upstream, promptText, completionText, "", "", 0)
}

func recordGatewayUsageWithTrace(profileName, model string, upstream UpstreamUsage, promptText, completionText, traceID, taskID string, durationMs float64) UsageInfo {
	info, estimated := resolveUsage(model, upstream, promptText, completionText)
	if !estimated && info.TotalTokens > 0 {
		quota.CalibrateTokens(model, promptText, info.PromptTokens, completionText, info.CompletionTokens)
	}
	if info.TotalTokens > 0 {
		_ = quota.RecordTokensWithTrace(profileName, quota.SourceGateway, model, info.PromptTokens, info.CompletionTokens, info.TotalTokens, estimated, traceID, taskID)
		telemetry.EmitTokenDelta(profileName, model, quota.SourceGateway, traceID, taskID, info.PromptTokens, info.CompletionTokens, info.TotalTokens, durationMs)
	}
	return info
}
