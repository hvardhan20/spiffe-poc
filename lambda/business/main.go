package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type respBody struct {
	Message  string `json:"message"`
	SPIFFEID string `json:"spiffe_id,omitempty"`
	Where    string `json:"where_found,omitempty"`
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// REST API + Lambda proxy integration commonly includes authorizer context here:
	// req.RequestContext.Authorizer["principalId"] or custom keys (e.g., "spiffe_id").
	// If you also map a header like X-Spiffe-Id, we'll check that too.

	spiffeID, where := findSPIFFEID(req)

	b, _ := json.Marshal(respBody{
		Message:  "hello from business lambda",
		SPIFFEID: spiffeID,
		Where:    where,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(b),
	}, nil
}

func findSPIFFEID(req events.APIGatewayProxyRequest) (string, string) {
	// 1) Check mapping header if you configured it in API Gateway integration request
	if v := req.Headers["X-Spiffe-Id"]; v != "" {
		return v, "headers.X-Spiffe-Id"
	}
	if v := req.Headers["x-spiffe-id"]; v != "" {
		return v, "headers.x-spiffe-id"
	}

	// 2) Check requestContext.authorizer fields
	if req.RequestContext.Authorizer != nil {
		// Some setups put principalId here
		if v, ok := req.RequestContext.Authorizer["principalId"].(string); ok && v != "" {
			return v, "requestContext.authorizer.principalId"
		}
		// Our authorizer context uses spiffe_id
		if v, ok := req.RequestContext.Authorizer["spiffe_id"].(string); ok && v != "" {
			return v, "requestContext.authorizer.spiffe_id"
		}
	}

	// 3) Fallback: nothing found
	return "", "not_found"
}

func main() { lambda.Start(handler) }
