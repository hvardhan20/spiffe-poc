package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type verifyReq struct {
	Token string `json:"token"`
}
type verifyResp struct {
	OK       bool                   `json:"ok"`
	SPIFFEID string                 `json:"spiffe_id"`
	Claims   map[string]interface{} `json:"claims"`
	Error    string                 `json:"error"`
}

var (
	verifierURL = env("VERIFIER_URL", "http://127.0.0.1:2773/verify")
	httpClient  = &http.Client{Timeout: 800 * time.Millisecond}
)

func handler(ctx context.Context, req events.APIGatewayCustomAuthorizerRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	token := extractBearer(req.Headers)
	if token == "" {
		return deny(req.MethodArn, "missing_bearer"), nil
	}

	spiffeID, err := verifyWithExtension(token)
	if err != nil {
		return deny(req.MethodArn, "invalid_token"), nil
	}

	return allow(req.MethodArn, spiffeID), nil
}

func extractBearer(h map[string]string) string {
	auth := h["Authorization"]
	if auth == "" {
		auth = h["authorization"]
	}
	auth = strings.TrimSpace(auth)
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[len("Bearer "):])
}

func verifyWithExtension(token string) (string, error) {
	b, _ := json.Marshal(verifyReq{Token: token})
	r, _ := http.NewRequest("POST", verifierURL, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(r)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var vr verifyResp
	_ = json.Unmarshal(body, &vr)

	if resp.StatusCode != 200 || !vr.OK || vr.SPIFFEID == "" {
		return "", fmt.Errorf("verify failed: %s", vr.Error)
	}
	return vr.SPIFFEID, nil
}

func allow(methodArn, spiffeID string) events.APIGatewayCustomAuthorizerResponse {
	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: spiffeID,
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   "Allow",
					Resource: []string{methodArn},
				},
			},
		},
		Context: map[string]interface{}{
			"spiffe_id": spiffeID,
		},
	}
}

func deny(methodArn, reason string) events.APIGatewayCustomAuthorizerResponse {
	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: "unauthorized",
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   "Deny",
					Resource: []string{methodArn},
				},
			},
		},
		Context: map[string]interface{}{
			"reason": reason,
		},
	}
}

func env(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func main() { lambda.Start(handler) }
