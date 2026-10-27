package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
)

// Deliver POSTs the result to the Phoenix callback URL with the
// job-scoped bearer token. A non-2xx response is an error: the caller
// exits non-zero so Phoenix classifies it as a worker crash rather than
// a reported result (see docs/ANALYSIS_CONTRACT.md).
func Deliver(ctx context.Context, client *http.Client, url, token string, res contract.Result) error {
	payload, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("callback request: %w", err)
	}
	defer resp.Body.Close()
	// Bound what we read back; Phoenix replies with a small ack.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("callback returned status %d", resp.StatusCode)
	}
	return nil
}
