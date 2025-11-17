package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type AdminStats struct {
	TotalRequests int64  `json:"total_requests"`
	Environment   string `json:"environment"`
	Service       string `json:"service"`
}

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("Admin service - Received %s request to %s", request.RequestContext.HTTP.Method, request.RawPath)

	// Extract user info from JWT claims
	userID := "unknown"
	roles := []string{}

	if sub, ok := request.RequestContext.Authorizer.JWT.Claims["sub"]; ok {
		userID = sub
	}

	// In production, check roles/permissions from JWT claims
	// For now, return mock admin data

	response := map[string]interface{}{
		"message": "Admin service - authenticated access",
		"user_id": userID,
		"roles":   roles,
		"stats": AdminStats{
			TotalRequests: 12345,
			Environment:   "dev",
			Service:       "admin-service",
		},
	}

	jsonBody, _ := json.Marshal(response)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(jsonBody),
	}, nil
}

func main() {
	lambda.Start(handler)
}
