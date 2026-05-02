package fineract

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type FineractClient interface {
	Transfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error)
	GetAccount(ctx context.Context, accountID string, accountType string) (*AccountResponse, error)
	Deposit(ctx context.Context, accountID string, accountType string, amount float64, note string) error
	Withdrawal(ctx context.Context, accountID string, accountType string, amount float64, note string) error
}

type TransferRequest struct {
	FromAccountID   string
	FromAccountType string
	ToAccountID     string
	ToAccountType   string
	Amount          float64
	Note            string
}

type TransferResponse struct {
	ResourceID int64
	Status     string
}

type AccountResponse struct {
	ID          int64
	AccountNo   string
	Status      map[string]interface{}
	Summary     map[string]interface{}
	AccountType map[string]interface{}
}

type fineractClient struct {
	baseURL    string
	username   string
	password   string
	tenantID   string
	httpClient *http.Client
}

func NewFineractClient(baseURL, username, password, tenantID string, timeoutSecs int) FineractClient {
	return &fineractClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		tenantID: tenantID,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSecs) * time.Second,
		},
	}
}

func (c *fineractClient) basicAuth() string {
	credentials := c.username + ":" + c.password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

func (c *fineractClient) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var buf *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.basicAuth())
	req.Header.Set("Fineract-Platform-TenantId", c.tenantID)
	return req, nil
}

func (c *fineractClient) Transfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error) {
	payload := map[string]interface{}{
		"fromAccountId":       req.FromAccountID,
		"fromAccountType":     accountTypeCode(req.FromAccountType),
		"toAccountId":         req.ToAccountID,
		"toAccountType":       accountTypeCode(req.ToAccountType),
		"transferAmount":      req.Amount,
		"transferDate":        time.Now().Format("02 January 2006"),
		"transferDescription": req.Note,
		"locale":              "en",
		"dateFormat":          "dd MMMM yyyy",
	}
	httpReq, err := c.newRequest(ctx, http.MethodPost, "/fineract-provider/api/v1/accounttransfers", payload)
	if err != nil {
		return nil, fmt.Errorf("create fineract transfer request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fineract transfer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("fineract transfer failed with status %d", resp.StatusCode)
	}

	var result TransferResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func (c *fineractClient) GetAccount(ctx context.Context, accountID string, accountType string) (*AccountResponse, error) {
	path := fmt.Sprintf("/fineract-provider/api/v1/%s/%s", accountTypePath(accountType), accountID)
	httpReq, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create fineract get account request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fineract get account: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fineract get account failed with status %d", resp.StatusCode)
	}

	var result AccountResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	return &result, nil
}

func (c *fineractClient) Deposit(ctx context.Context, accountID string, accountType string, amount float64, note string) error {
	path := fmt.Sprintf("/fineract-provider/api/v1/%s/%s/transactions?command=deposit", accountTypePath(accountType), accountID)
	payload := map[string]interface{}{
		"transactionDate":   time.Now().Format("02 January 2006"),
		"transactionAmount": amount,
		"paymentTypeId":     1,
		"note":              note,
		"locale":            "en",
		"dateFormat":        "dd MMMM yyyy",
	}
	httpReq, err := c.newRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("create fineract deposit request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("fineract deposit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("fineract deposit failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *fineractClient) Withdrawal(ctx context.Context, accountID string, accountType string, amount float64, note string) error {
	path := fmt.Sprintf("/fineract-provider/api/v1/%s/%s/transactions?command=withdrawal", accountTypePath(accountType), accountID)
	payload := map[string]interface{}{
		"transactionDate":   time.Now().Format("02 January 2006"),
		"transactionAmount": amount,
		"paymentTypeId":     1,
		"note":              note,
		"locale":            "en",
		"dateFormat":        "dd MMMM yyyy",
	}
	httpReq, err := c.newRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("create fineract withdrawal request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("fineract withdrawal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("fineract withdrawal failed with status %d", resp.StatusCode)
	}
	return nil
}

func accountTypePath(t string) string {
	switch t {
	case "savings":
		return "savingsaccounts"
	case "loans":
		return "loans"
	default:
		return "savingsaccounts"
	}
}

func accountTypeCode(t string) int {
	switch t {
	case "savings":
		return 2
	case "loans":
		return 1
	default:
		return 2
	}
}
