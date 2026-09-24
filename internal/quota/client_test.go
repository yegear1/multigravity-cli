package quota

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFormatProgressBar(t *testing.T) {
	bar0 := FormatProgressBar(0, 20)
	if bar0 != "[                    ]" {
		t.Errorf("expected empty bar, got %s", bar0)
	}

	bar100 := FormatProgressBar(100, 20)
	if bar100 != "[====================]" {
		t.Errorf("expected full bar, got %s", bar100)
	}

	bar50 := FormatProgressBar(50, 20)
	if bar50 != "[==========          ]" {
		t.Errorf("expected half bar, got %s", bar50)
	}
}

func TestFormatResetCountdown(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	future := time.Date(2026, 9, 24, 14, 30, 0, 0, time.UTC)

	resStr, secs := FormatResetCountdown(future.Format(time.RFC3339), now)
	if secs != 9000 {
		t.Errorf("expected 9000s, got %d", secs)
	}
	if !strings.Contains(resStr, "2h 30m") {
		t.Errorf("expected '2h 30m', got %s", resStr)
	}

	past := time.Date(2026, 9, 24, 11, 0, 0, 0, time.UTC)
	pastStr, pastSecs := FormatResetCountdown(past.Format(time.RFC3339), now)
	if pastSecs > 0 || !strings.Contains(pastStr, "Quota refreshed!") {
		t.Errorf("expected 'Quota refreshed!', got %s", pastStr)
	}
}

func TestClientQuotaAndCascade(t *testing.T) {
	expectedCSRF := "test-csrf-token"
	expectedCascadeID := "cascade-1234"

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csrf := r.Header.Get("X-Codeium-Csrf-Token")
		if csrf != expectedCSRF {
			http.Error(w, "invalid csrf", http.StatusForbidden)
			return
		}

		switch r.URL.Path {
		case "/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary":
			resp := QuotaSummaryResponse{
				Response: QuotaResponse{
					Groups: []QuotaGroup{
						{
							DisplayName: "Gemini",
							Description: "Gemini Quota",
							Buckets: []QuotaBucket{
								{
									BucketID:          "gemini-weekly",
									DisplayName:       "Weekly Limit",
									RemainingFraction: 0.85,
									ResetTime:         "2026-09-30T00:00:00Z",
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case "/exa.language_server_pb.LanguageServerService/StartCascade":
			var startReq StartCascadeRequest
			_ = json.NewDecoder(r.Body).Decode(&startReq)
			if startReq.Source != "CORTEX_TRAJECTORY_SOURCE_CLI" {
				http.Error(w, "invalid source", http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(StartCascadeResponse{CascadeID: expectedCascadeID})

		case "/exa.language_server_pb.LanguageServerService/SendUserCascadeMessage":
			var msgReq SendUserCascadeMessageRequest
			_ = json.NewDecoder(r.Body).Decode(&msgReq)
			if msgReq.CascadeID != expectedCascadeID || len(msgReq.Items) == 0 {
				http.Error(w, "invalid msg req", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())

	client := NewClient(2 * time.Second)

	// 1. Test RetrieveUserQuotaSummary
	quotaSummary, err := client.RetrieveUserQuotaSummary(port, expectedCSRF)
	if err != nil {
		t.Fatalf("RetrieveUserQuotaSummary failed: %v", err)
	}
	if len(quotaSummary.Response.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(quotaSummary.Response.Groups))
	}
	if quotaSummary.Response.Groups[0].Buckets[0].RemainingFraction != 0.85 {
		t.Errorf("expected 0.85 remaining, got %f", quotaSummary.Response.Groups[0].Buckets[0].RemainingFraction)
	}

	// 2. Test StartCascade
	cascadeID, err := client.StartCascade(port, expectedCSRF)
	if err != nil {
		t.Fatalf("StartCascade failed: %v", err)
	}
	if cascadeID != expectedCascadeID {
		t.Errorf("expected %s, got %s", expectedCascadeID, cascadeID)
	}

	// 3. Test SendUserCascadeMessage
	err = client.SendUserCascadeMessage(port, expectedCSRF, cascadeID, "hello ping", "MODEL_PLACEHOLDER_M73")
	if err != nil {
		t.Fatalf("SendUserCascadeMessage failed: %v", err)
	}

	// 4. Test RenderQuotaStatus
	var buf bytes.Buffer
	RenderQuotaStatus(&buf, []ActiveServer{
		{
			Profile: "dev-test",
			PID:     1234,
			Port:    port,
			CSRF:    expectedCSRF,
			Data:    quotaSummary,
		},
	})
	rendered := buf.String()
	if !strings.Contains(rendered, "dev-test") || !strings.Contains(rendered, "85.0%") {
		t.Errorf("expected output to contain profile and percentage, got:\n%s", rendered)
	}
}

func TestGenerateUUID(t *testing.T) {
	u1 := GenerateUUID()
	u2 := GenerateUUID()
	if len(u1) != 36 || len(u2) != 36 {
		t.Errorf("invalid UUID lengths: %s, %s", u1, u2)
	}
	if u1 == u2 {
		t.Errorf("UUIDs should be distinct: %s vs %s", u1, u2)
	}
}
