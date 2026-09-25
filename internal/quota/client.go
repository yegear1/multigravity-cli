package quota

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

func GenerateUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (c *Client) RetrieveUserQuotaSummary(port int, csrf string) (*QuotaSummaryResponse, error) {
	url := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary", port)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Codeium-Csrf-Token", csrf)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var quotaResp QuotaSummaryResponse
	if err := json.Unmarshal(body, &quotaResp); err != nil {
		return nil, fmt.Errorf("failed to parse quota summary: %w", err)
	}

	now := time.Now().UTC()
	for i := range quotaResp.Response.Groups {
		for j := range quotaResp.Response.Groups[i].Buckets {
			if quotaResp.Response.Groups[i].Buckets[j].WindowType == "" {
				quotaResp.Response.Groups[i].Buckets[j].WindowType = ClassifyWindow(quotaResp.Response.Groups[i].Buckets[j], now)
			}
		}
	}

	return &quotaResp, nil
}

func (c *Client) StartCascade(port int, csrf string) (string, error) {
	url := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/StartCascade", port)
	reqBody, err := json.Marshal(StartCascadeRequest{
		Source: "CORTEX_TRAJECTORY_SOURCE_CLI",
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Codeium-Csrf-Token", csrf)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("StartCascade returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var startResp StartCascadeResponse
	if err := json.Unmarshal(body, &startResp); err != nil {
		return "", fmt.Errorf("failed to parse StartCascade response: %w", err)
	}

	return startResp.CascadeID, nil
}

func (c *Client) SendUserCascadeMessage(port int, csrf string, cascadeID string, prompt string, model string) error {
	url := fmt.Sprintf("https://127.0.0.1:%d/exa.language_server_pb.LanguageServerService/SendUserCascadeMessage", port)
	payload := SendUserCascadeMessageRequest{
		CascadeID: cascadeID,
		Items: []MessageItem{
			{Text: prompt},
		},
		CascadeConfig: CascadeConfig{
			PlannerConfig: PlannerConfig{
				RequestedModel: RequestedModel{
					Model: model,
				},
			},
		},
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Codeium-Csrf-Token", csrf)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SendUserCascadeMessage returned status %d: %s", resp.StatusCode, string(body))
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
