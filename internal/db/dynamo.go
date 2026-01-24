// Package db provides DynamoDB client and helper functions for database operations.
package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Client wraps the DynamoDB client with table name configuration.
type Client struct {
	db        *dynamodb.Client
	tableName string
}

// NewClient creates a new DynamoDB client initialized from environment configuration.
func NewClient(ctx context.Context) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	tableName := os.Getenv("TABLE_NAME")
	if tableName == "" {
		tableName = "BookingPlatformTable"
	}

	return &Client{
		db:        dynamodb.NewFromConfig(cfg),
		tableName: tableName,
	}, nil
}

// TableName returns the configured table name.
func (c *Client) TableName() string {
	return c.tableName
}

// PutItem stores an item in DynamoDB.
func (c *Client) PutItem(ctx context.Context, item interface{}) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

// PutItemWithCondition stores an item with a condition expression.
func (c *Client) PutItemWithCondition(ctx context.Context, item interface{}, condition string) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(c.tableName),
		Item:                av,
		ConditionExpression: aws.String(condition),
	})
	if err != nil {
		return fmt.Errorf("failed to put item with condition: %w", err)
	}

	return nil
}

// GetItem retrieves an item by primary key.
func (c *Client) GetItem(ctx context.Context, pk, sk string, out interface{}) error {
	result, err := c.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get item: %w", err)
	}

	if result.Item == nil {
		return ErrNotFound
	}

	if err := attributevalue.UnmarshalMap(result.Item, out); err != nil {
		return fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return nil
}

// Query executes a query on the main table or GSI.
func (c *Client) Query(ctx context.Context, params QueryParams) ([]map[string]types.AttributeValue, error) {
	exprValues := make(map[string]types.AttributeValue)
	for k, v := range params.ExpressionValues {
		av, err := attributevalue.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal expression value %s: %w", k, err)
		}
		exprValues[k] = av
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(c.tableName),
		KeyConditionExpression:    aws.String(params.KeyCondition),
		ExpressionAttributeValues: exprValues,
	}

	if params.IndexName != "" {
		input.IndexName = aws.String(params.IndexName)
	}

	if params.FilterExpression != "" {
		input.FilterExpression = aws.String(params.FilterExpression)
	}

	if params.Limit > 0 {
		input.Limit = aws.Int32(params.Limit)
	}

	if params.ScanIndexForward != nil {
		input.ScanIndexForward = params.ScanIndexForward
	}

	result, err := c.db.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}

	return result.Items, nil
}

// QueryParams holds parameters for a DynamoDB query.
type QueryParams struct {
	KeyCondition     string
	FilterExpression string
	ExpressionValues map[string]interface{}
	IndexName        string
	Limit            int32
	ScanIndexForward *bool
}

// Scan executes a scan on the table.
func (c *Client) Scan(ctx context.Context, params ScanParams) ([]map[string]types.AttributeValue, error) {
	exprValues := make(map[string]types.AttributeValue)
	for k, v := range params.ExpressionValues {
		av, err := attributevalue.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal expression value %s: %w", k, err)
		}
		exprValues[k] = av
	}

	input := &dynamodb.ScanInput{
		TableName: aws.String(c.tableName),
	}

	if params.FilterExpression != "" {
		input.FilterExpression = aws.String(params.FilterExpression)
		if len(exprValues) > 0 {
			input.ExpressionAttributeValues = exprValues
		}
	}

	if params.IndexName != "" {
		input.IndexName = aws.String(params.IndexName)
	}

	if params.Limit > 0 {
		input.Limit = aws.Int32(params.Limit)
	}

	result, err := c.db.Scan(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to scan: %w", err)
	}

	return result.Items, nil
}

// ScanParams holds parameters for a DynamoDB scan.
type ScanParams struct {
	FilterExpression string
	ExpressionValues map[string]interface{}
	IndexName        string
	Limit            int32
}

// UpdateItem updates an item in DynamoDB.
func (c *Client) UpdateItem(ctx context.Context, pk, sk string, params UpdateParams) error {
	exprValues := make(map[string]types.AttributeValue)
	for k, v := range params.ExpressionValues {
		av, err := attributevalue.Marshal(v)
		if err != nil {
			return fmt.Errorf("failed to marshal expression value %s: %w", k, err)
		}
		exprValues[k] = av
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression:          aws.String(params.UpdateExpression),
		ExpressionAttributeValues: exprValues,
	}

	if params.ConditionExpression != "" {
		input.ConditionExpression = aws.String(params.ConditionExpression)
	}

	if len(params.ExpressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = params.ExpressionAttributeNames
	}

	_, err := c.db.UpdateItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// UpdateParams holds parameters for a DynamoDB update.
type UpdateParams struct {
	UpdateExpression         string
	ConditionExpression      string
	ExpressionValues         map[string]interface{}
	ExpressionAttributeNames map[string]string
}

// DeleteItem removes an item from DynamoDB.
func (c *Client) DeleteItem(ctx context.Context, pk, sk string) error {
	_, err := c.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}

// CalculateTTL returns a Unix timestamp for TTL expiration.
func CalculateTTL(duration time.Duration) int64 {
	return time.Now().Add(duration).Unix()
}

// ErrNotFound is returned when an item is not found in DynamoDB.
var ErrNotFound = fmt.Errorf("item not found")

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return err == ErrNotFound
}
