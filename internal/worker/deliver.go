package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Patchwrit/patchwrit-worker/internal/contract"
	"github.com/Patchwrit/patchwrit-worker/internal/observe"
)

// MaxCallbackPayloadBytes mirrors the Phoenix callback limit
// (docs/ANALYSIS_CONTRACT.md). Oversize results fail explicitly here
// instead of being rejected opaquely at delivery (review W009).
const MaxCallbackPayloadBytes = 4 * 1024 * 1024

// Deliver POSTs the result to the Phoenix callback URL with the
// job-scoped bearer token. A non-2xx response is an error: the caller
// exits non-zero so Phoenix classifies it as a worker crash rather than
// a reported result (see docs/ANALYSIS_CONTRACT.md).
func Deliver(ctx context.Context, client *http.Client, url, token string, res contract.Result) error {
	payload, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	if len(payload) > MaxCallbackPayloadBytes {
		return fmt.Errorf("result payload is %d bytes, exceeds callback limit %d", len(payload), MaxCallbackPayloadBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	observe.Event(res.JobID, "callback.started", map[string]any{
		"payload_bytes": len(payload),
		"status":        res.Status,
		"dependencies":  len(res.Dependencies),
	})

	resp, err := client.Do(req)
	if err != nil {
		observe.Event(res.JobID, "callback.failed", map[string]any{"error": err.Error()})
		return fmt.Errorf("callback request: %w", err)
	}
	defer resp.Body.Close()
	// Bound what we read back; Phoenix replies with a small ack.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		observe.Event(res.JobID, "callback.failed", map[string]any{"http_status": resp.StatusCode})
		return fmt.Errorf("callback returned status %d", resp.StatusCode)
	}
	observe.Event(res.JobID, "callback.completed", map[string]any{"http_status": resp.StatusCode})
	return nil
}
