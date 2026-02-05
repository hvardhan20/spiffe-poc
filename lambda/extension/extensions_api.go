package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type extClient struct {
	runtimeAPI string
	extID      string
	http       *http.Client
}

func newExtClient() (*extClient, error) {
	runtimeAPI := os.Getenv("AWS_LAMBDA_RUNTIME_API")
	if runtimeAPI == "" {
		return nil, fmt.Errorf("AWS_LAMBDA_RUNTIME_API not set (not running in Lambda?)")
	}
	return &extClient{
		runtimeAPI: runtimeAPI,
		http:       &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (c *extClient) register() error {
	url := fmt.Sprintf("http://%s/2020-01-01/extension/register", c.runtimeAPI)
	payload := map[string]any{"events": []string{"INVOKE", "SHUTDOWN"}}
	b, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Lambda-Extension-Name", "spiffe-verifier")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("register failed: %s: %s", resp.Status, string(body))
	}

	// The extension identifier must be used in subsequent /event/next calls.
	extID := resp.Header.Get("Lambda-Extension-Identifier")
	if extID == "" {
		return fmt.Errorf("missing Lambda-Extension-Identifier header")
	}
	c.extID = extID
	return nil
}

func (c *extClient) nextEvent(ctx context.Context) ([]byte, error) {
	url := fmt.Sprintf("http://%s/2020-01-01/extension/event/next", c.runtimeAPI)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Lambda-Extension-Identifier", c.extID)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("event/next failed: %s: %s", resp.Status, string(body))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
