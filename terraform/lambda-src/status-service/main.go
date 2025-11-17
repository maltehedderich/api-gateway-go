package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type HealthResponse struct {
	Status      string    `json:"status"`
	Environment string    `json:"environment"`
	Timestamp   time.Time `json:"timestamp"`
	Service     string    `json:"service"`
}

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("Health check - Received %s request to %s", request.RequestContext.HTTP.Method, request.RawPath)

	response := HealthResponse{
		Status:      "healthy",
		Environment: "dev",
		Timestamp:   time.Now(),
		Service:     "status-service",
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
