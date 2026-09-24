package quota

type QuotaBucket struct {
	BucketID          string  `json:"bucketId"`
	DisplayName       string  `json:"displayName"`
	Description       string  `json:"description"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

type QuotaGroup struct {
	DisplayName string        `json:"displayName"`
	Description string        `json:"description"`
	Buckets     []QuotaBucket `json:"buckets"`
}

type QuotaResponse struct {
	Groups []QuotaGroup `json:"groups"`
}

type QuotaSummaryResponse struct {
	Response QuotaResponse `json:"response"`
}

type ActiveServer struct {
	Profile string                `json:"profile"`
	PID     int                   `json:"pid"`
	Port    int                   `json:"port"`
	CSRF    string                `json:"csrf,omitempty"`
	Data    *QuotaSummaryResponse `json:"data,omitempty"`
}

type StartCascadeRequest struct {
	Source string `json:"source"`
}

type StartCascadeResponse struct {
	CascadeID string `json:"cascadeId"`
}

type RequestedModel struct {
	Model string `json:"model"`
}

type PlannerConfig struct {
	RequestedModel RequestedModel `json:"requestedModel"`
}

type CascadeConfig struct {
	PlannerConfig PlannerConfig `json:"plannerConfig"`
}

type MessageItem struct {
	Text string `json:"text"`
}

type SendUserCascadeMessageRequest struct {
	CascadeID     string        `json:"cascadeId"`
	Items         []MessageItem `json:"items"`
	CascadeConfig CascadeConfig `json:"cascadeConfig"`
}
