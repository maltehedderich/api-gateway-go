package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

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

type Order struct {
	OrderID   string  `json:"order_id" dynamodbav:"order_id"`
	UserID    string  `json:"user_id" dynamodbav:"user_id"`
	Product   string  `json:"product" dynamodbav:"product"`
	Quantity  int     `json:"quantity" dynamodbav:"quantity"`
	Total     float64 `json:"total" dynamodbav:"total"`
	Status    string  `json:"status" dynamodbav:"status"`
	CreatedAt int64   `json:"created_at" dynamodbav:"created_at"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load AWS SDK config: %v", err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg)

	tableName = os.Getenv("DYNAMODB_TABLE_ORDERS")
	if tableName == "" {
		tableName = "api-gateway-dev-orders"
	}

	environment = os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "dev"
	}

	log.Printf("Order service initialized - Environment: %s, Table: %s", environment, tableName)
}

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	log.Printf("Received %s request to %s", request.RequestContext.HTTP.Method, request.RawPath)

	routeKey := request.RouteKey

	switch routeKey {
	case "GET /api/v1/orders":
		return listOrders(ctx, request)
	case "POST /api/v1/orders":
		return createOrder(ctx, request)
	case "GET /api/v1/orders/{id}":
		return getOrder(ctx, request)
	case "PUT /api/v1/orders/{id}":
		return updateOrder(ctx, request)
	case "DELETE /api/v1/orders/{id}":
		return deleteOrder(ctx, request)
	default:
		return errorResponse(404, "Route not found"), nil
	}
}

func listOrders(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	result, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(100),
	})
	if err != nil {
		log.Printf("Failed to scan orders: %v", err)
		return errorResponse(500, "Failed to list orders"), nil
	}

	var orders []Order
	err = attributevalue.UnmarshalListOfMaps(result.Items, &orders)
	if err != nil {
		log.Printf("Failed to unmarshal orders: %v", err)
		return errorResponse(500, "Failed to parse orders"), nil
	}

	return jsonResponse(200, map[string]interface{}{
		"orders": orders,
		"count":  len(orders),
	}), nil
}

func createOrder(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	var input struct {
		Product  string  `json:"product"`
		Quantity int     `json:"quantity"`
		Total    float64 `json:"total"`
	}

	if err := json.Unmarshal([]byte(request.Body), &input); err != nil {
		return errorResponse(400, "Invalid request body"), nil
	}

	// Get user ID from JWT claims (passed by API Gateway authorizer)
	userID := "anonymous"
	if claims, ok := request.RequestContext.Authorizer.JWT.Claims["sub"]; ok {
		userID = claims
	}

	order := Order{
		OrderID:   uuid.New().String(),
		UserID:    userID,
		Product:   input.Product,
		Quantity:  input.Quantity,
		Total:     input.Total,
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}

	item, err := attributevalue.MarshalMap(order)
	if err != nil {
		log.Printf("Failed to marshal order: %v", err)
		return errorResponse(500, "Failed to create order"), nil
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		log.Printf("Failed to put order: %v", err)
		return errorResponse(500, "Failed to save order"), nil
	}

	return jsonResponse(201, order), nil
}

func getOrder(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	orderID := request.PathParameters["id"]
	if orderID == "" {
		return errorResponse(400, "Order ID is required"), nil
	}

	result, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"order_id": &types.AttributeValueMemberS{Value: orderID},
		},
	})
	if err != nil {
		log.Printf("Failed to get order: %v", err)
		return errorResponse(500, "Failed to retrieve order"), nil
	}

	if result.Item == nil {
		return errorResponse(404, "Order not found"), nil
	}

	var order Order
	err = attributevalue.UnmarshalMap(result.Item, &order)
	if err != nil {
		log.Printf("Failed to unmarshal order: %v", err)
		return errorResponse(500, "Failed to parse order"), nil
	}

	return jsonResponse(200, order), nil
}

func updateOrder(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	orderID := request.PathParameters["id"]
	if orderID == "" {
		return errorResponse(400, "Order ID is required"), nil
	}

	var input struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal([]byte(request.Body), &input); err != nil {
		return errorResponse(400, "Invalid request body"), nil
	}

	_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"order_id": &types.AttributeValueMemberS{Value: orderID},
		},
		UpdateExpression: aws.String("SET #status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: input.Status},
		},
	})
	if err != nil {
		log.Printf("Failed to update order: %v", err)
		return errorResponse(500, "Failed to update order"), nil
	}

	return jsonResponse(200, map[string]string{
		"message":  "Order updated successfully",
		"order_id": orderID,
	}), nil
}

func deleteOrder(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	orderID := request.PathParameters["id"]
	if orderID == "" {
		return errorResponse(400, "Order ID is required"), nil
	}

	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"order_id": &types.AttributeValueMemberS{Value: orderID},
		},
	})
	if err != nil {
		log.Printf("Failed to delete order: %v", err)
		return errorResponse(500, "Failed to delete order"), nil
	}

	return jsonResponse(200, map[string]string{
		"message":  "Order deleted successfully",
		"order_id": orderID,
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

func errorResponse(statusCode int, message string) events.APIGatewayV2HTTPResponse {
	return jsonResponse(statusCode, ErrorResponse{
		Error:   "error",
		Message: message,
	})
}

func main() {
	lambda.Start(handler)
}
