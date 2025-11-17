package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

var (
	dynamoClient *dynamodb.Client
	tableName    string
	environment  string
)

type User struct {
	UserID    string `json:"user_id" dynamodbav:"user_id"`
	Email     string `json:"email" dynamodbav:"email"`
	Name      string `json:"name" dynamodbav:"name"`
	CreatedAt int64  `json:"created_at" dynamodbav:"created_at"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func init() {
	// Initialize AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load AWS SDK config: %v", err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg)

	// Get configuration from environment
	tableName = os.Getenv("DYNAMODB_TABLE_USERS")
	if tableName == "" {
		tableName = "api-gateway-dev-users" // Default for development
	}

	environment = os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "dev"
	}

	log.Printf("User service initialized - Environment: %s, Table: %s", environment, tableName)
}

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("Received %s request to %s", request.RequestContext.HTTP.Method, request.RawPath)

	// Route based on HTTP method and path
	method := request.RequestContext.HTTP.Method
	routeKey := request.RouteKey

	switch routeKey {
	case "GET /api/v1/users":
		return listUsers(ctx, request)
	case "POST /api/v1/users":
		return createUser(ctx, request)
	case "GET /api/v1/users/{id}":
		return getUser(ctx, request)
	case "PUT /api/v1/users/{id}":
		return updateUser(ctx, request)
	case "DELETE /api/v1/users/{id}":
		return deleteUser(ctx, request)
	default:
		return errorResponse(404, "Route not found", ""), nil
	}
}

func listUsers(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Scan DynamoDB table (for production, use Query with pagination)
	result, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(100), // Limit results
	})
	if err != nil {
		log.Printf("Failed to scan users: %v", err)
		return errorResponse(500, "Failed to list users", err.Error()), nil
	}

	var users []User
	err = attributevalue.UnmarshalListOfMaps(result.Items, &users)
	if err != nil {
		log.Printf("Failed to unmarshal users: %v", err)
		return errorResponse(500, "Failed to parse users", err.Error()), nil
	}

	return jsonResponse(200, map[string]interface{}{
		"users": users,
		"count": len(users),
	}), nil
}

func createUser(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	var input struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := json.Unmarshal([]byte(request.Body), &input); err != nil {
		return errorResponse(400, "Invalid request body", err.Error()), nil
	}

	// Validate input
	if input.Email == "" || input.Name == "" {
		return errorResponse(400, "Email and name are required", ""), nil
	}

	// Create user
	user := User{
		UserID:    uuid.New().String(),
		Email:     input.Email,
		Name:      input.Name,
		CreatedAt: 0, // Set to current timestamp in production
	}

	// Marshal to DynamoDB item
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		log.Printf("Failed to marshal user: %v", err)
		return errorResponse(500, "Failed to create user", err.Error()), nil
	}

	// Put item in DynamoDB
	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("Failed to put user: %v", err)
		return errorResponse(500, "Failed to save user", err.Error()), nil
	}

	return jsonResponse(201, user), nil
}

func getUser(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	userID := request.PathParameters["id"]
	if userID == "" {
		return errorResponse(400, "User ID is required", ""), nil
	}

	// Get item from DynamoDB
	result, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		log.Printf("Failed to get user: %v", err)
		return errorResponse(500, "Failed to retrieve user", err.Error()), nil
	}

	if result.Item == nil {
		return errorResponse(404, "User not found", ""), nil
	}

	var user User
	err = attributevalue.UnmarshalMap(result.Item, &user)
	if err != nil {
		log.Printf("Failed to unmarshal user: %v", err)
		return errorResponse(500, "Failed to parse user", err.Error()), nil
	}

	return jsonResponse(200, user), nil
}

func updateUser(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	userID := request.PathParameters["id"]
	if userID == "" {
		return errorResponse(400, "User ID is required", ""), nil
	}

	var input struct {
		Name string `json:"name"`
	}

	if err := json.Unmarshal([]byte(request.Body), &input); err != nil {
		return errorResponse(400, "Invalid request body", err.Error()), nil
	}

	// Update item in DynamoDB
	_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET #name = :name"),
		ExpressionAttributeNames: map[string]string{
			"#name": "name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: input.Name},
		},
	})
	if err != nil {
		log.Printf("Failed to update user: %v", err)
		return errorResponse(500, "Failed to update user", err.Error()), nil
	}

	return jsonResponse(200, map[string]string{
		"message": "User updated successfully",
		"user_id": userID,
	}), nil
}

func deleteUser(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	userID := request.PathParameters["id"]
	if userID == "" {
		return errorResponse(400, "User ID is required", ""), nil
	}

	// Delete item from DynamoDB
	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		log.Printf("Failed to delete user: %v", err)
		return errorResponse(500, "Failed to delete user", err.Error()), nil
	}

	return jsonResponse(200, map[string]string{
		"message": "User deleted successfully",
		"user_id": userID,
	}), nil
}

func jsonResponse(statusCode int, body interface{}) events.APIGatewayV2HTTPResponse {
	jsonBody, _ := json.Marshal(body)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(jsonBody),
	}
}

func errorResponse(statusCode int, message string, details string) events.APIGatewayV2HTTPResponse {
	errResp := ErrorResponse{
		Error:   "error",
		Message: message,
	}
	if details != "" && environment == "dev" {
		errResp.Message = fmt.Sprintf("%s: %s", message, details)
	}
	return jsonResponse(statusCode, errResp)
}

func main() {
	lambda.Start(handler)
}
