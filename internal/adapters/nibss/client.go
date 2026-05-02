package nibss

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type NIBSSClient interface {
	InitiateTransfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error)
	GetTransferStatus(ctx context.Context, sessionID string) (*TransferStatus, error)
}

type TransferRequest struct {
	SessionID     string
	SourceAccount string
	DestAccount   string
	DestBankCode  string
	Amount        float64
	Currency      string
	Narration     string
}

type TransferResponse struct {
	SessionID string
	Status    string
	Message   string
	Reference string
}

type TransferStatus struct {
	SessionID string
	Status    string
	Message   string
}

type nibssClient struct {
	baseURL    string
	apiKey     string
	secretKey  string
	httpClient *http.Client
}

func NewNIBSSClient(baseURL, apiKey, secretKey string, timeoutSecs int) NIBSSClient {
	return &nibssClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
	}
}

func (c *nibssClient) sign(payload []byte) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *nibssClient) InitiateTransfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error) {
	payload, _ := json.Marshal(req)
	signature := c.sign(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/transfer", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create NIBSS request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)
	httpReq.Header.Set("Authorization", "HMAC "+signature)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("NIBSS transfer request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("NIBSS transfer failed with status %d", resp.StatusCode)
	}

	var result TransferResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode NIBSS response: %w", err)
	}
	return &result, nil
}

func (c *nibssClient) GetTransferStatus(ctx context.Context, sessionID string) (*TransferStatus, error) {
	url := fmt.Sprintf("%s/transfer/%s/status", c.baseURL, sessionID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create NIBSS status request: %w", err)
	}
	httpReq.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("NIBSS status request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NIBSS status check failed with status %d", resp.StatusCode)
	}

	var result TransferStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode NIBSS status response: %w", err)
	}
	return &result, nil
}
