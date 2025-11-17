package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBStorage implements Storage interface using AWS DynamoDB
type DynamoDBStorage struct {
	client    *dynamodb.Client
	tableName string
}

// dynamoDBItem represents a rate limit entry in DynamoDB
type dynamoDBItem struct {
	Key        string  `dynamodbav:"key"`        // Partition key
	Window     string  `dynamodbav:"window"`     // Sort key (time window identifier)
	Capacity   float64 `dynamodbav:"capacity"`   // Bucket capacity
	RefillRate float64 `dynamodbav:"refill_rate"` // Tokens per second
	Tokens     float64 `dynamodbav:"tokens"`     // Available tokens
	LastRefill int64   `dynamodbav:"last_refill"` // Last refill timestamp (Unix)
	ExpiresAt  int64   `dynamodbav:"expires_at"` // TTL for automatic cleanup
}

// NewDynamoDBStorage creates a new DynamoDB-backed rate limit storage
func NewDynamoDBStorage(tableName, region string) (Storage, error) {
	// Load AWS SDK configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	return &DynamoDBStorage{
		client:    client,
		tableName: tableName,
	}, nil
}

// Get retrieves rate limit state from DynamoDB
func (d *DynamoDBStorage) Get(ctx context.Context, key string) (*BucketState, bool, error) {
	window := d.getCurrentWindow()

	input := &dynamodb.GetItemInput{
		TableName: aws.String(d.tableName),
		Key: map[string]types.AttributeValue{
			"key":    &types.AttributeValueMemberS{Value: key},
			"window": &types.AttributeValueMemberS{Value: window},
		},
		ConsistentRead: aws.Bool(true),
	}

	result, err := d.client.GetItem(ctx, input)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get item from DynamoDB: %w", err)
	}

	// If item doesn't exist, return nil (not an error)
	if result.Item == nil {
		return nil, false, nil
	}

	// Unmarshal DynamoDB item
	var item dynamoDBItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal DynamoDB item: %w", err)
	}

	state := &BucketState{
		Capacity:   item.Capacity,
		RefillRate: item.RefillRate,
		Tokens:     item.Tokens,
		LastRefill: time.Unix(item.LastRefill, 0),
	}

	return state, true, nil
}

// Set stores rate limit state in DynamoDB
func (d *DynamoDBStorage) Set(ctx context.Context, key string, state *BucketState, ttl time.Duration) error {
	window := d.getCurrentWindow()
	expiresAt := time.Now().Add(ttl).Unix()

	item := dynamoDBItem{
		Key:        key,
		Window:     window,
		Capacity:   state.Capacity,
		RefillRate: state.RefillRate,
		Tokens:     state.Tokens,
		LastRefill: state.LastRefill.Unix(),
		ExpiresAt:  expiresAt,
	}

	// Marshal to DynamoDB attribute values
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal DynamoDB item: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName),
		Item:      av,
	}

	_, err = d.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put item to DynamoDB: %w", err)
	}

	return nil
}

// Close closes the DynamoDB client
func (d *DynamoDBStorage) Close() error {
	// DynamoDB client doesn't need explicit closing
	return nil
}

// Ping checks if DynamoDB is accessible
func (d *DynamoDBStorage) Ping(ctx context.Context) error {
	// Try to describe the table
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(d.tableName),
	}

	_, err := d.client.DescribeTable(ctx, input)
	if err != nil {
		return fmt.Errorf("DynamoDB health check failed: %w", err)
	}

	return nil
}

// getCurrentWindow returns the current time window identifier (rounded to minute)
// This allows rate limits to be scoped to time windows
func (d *DynamoDBStorage) getCurrentWindow() string {
	now := time.Now()
	// Round down to the current minute
	window := now.Truncate(time.Minute)
	return window.Format("2006-01-02T15:04")
}
