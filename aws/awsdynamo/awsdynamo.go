// nolint:typecheck
package awsdynamo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	awstime "github.com/aws/smithy-go/time"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awsdynamo/dynamooptions"
)

var (
	ErrNotFound = errors.New("record not found")
)

// PutItem Put the item in DynamoDB Upsert if it does not exist
//
// Type parameters:
//   - T: the type of the item to retrieve
func PutItem[T any](
	ctx context.Context,
	region awsconfig.Region,
	tableName TableName,
	item T,
	opts ...dynamooptions.OptionDynamo,
) error {
	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return err
	}
	putItem, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	putItemInput := &dynamodb.PutItemInput{
		Item:      putItem,
		TableName: tableName.AWSString(),
	}
	if _, err := client.PutItem(ctx, putItemInput); err != nil {
		return err
	}
	return nil
}

// UpdateItem Update the attributes of the item in DynamoDB Upsert if it does not exist
//
// Type parameters:
//   - T: the type of the item to retrieve
//   - K: a string-compatible type for the key value
//
// Returns the retrieved item or ErrNotFound if the item doesn't exist.
func UpdateItem[T any, K ~string](
	ctx context.Context,
	region awsconfig.Region,
	tableName TableName,
	keyAttributeName KeyAttributeName,
	key K,
	update expression.UpdateBuilder,
	opts ...dynamooptions.OptionDynamo,
) (*T, error) {
	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return nil, err
	}
	expr, err := expression.NewBuilder().WithUpdate(update).Build()
	if err != nil {
		return nil, err
	}
	updateItemInput := &dynamodb.UpdateItemInput{
		Key: map[string]types.AttributeValue{
			keyAttributeName.String(): &types.AttributeValueMemberS{Value: string(key)},
		},
		TableName:                   tableName.AWSString(),
		ExpressionAttributeNames:    expr.Names(),
		ExpressionAttributeValues:   expr.Values(),
		ReturnConsumedCapacity:      types.ReturnConsumedCapacityNone,
		ReturnItemCollectionMetrics: types.ReturnItemCollectionMetricsNone,
		ReturnValues:                types.ReturnValueAllNew,
		UpdateExpression:            expr.Update(),
	}
	updatedItem, err := client.UpdateItem(ctx, updateItemInput)
	if err != nil {
		return nil, err
	}
	if updatedItem.Attributes == nil {
		return nil, ErrNotFound
	}
	out := new(T)
	if err := attributevalue.UnmarshalMap(updatedItem.Attributes, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteItem Delete DynamoDB item
//
// Type parameters:
//   - T: the type of the item to retrieve
//   - K: a string-compatible type for the key value
//
// Returns the retrieved item or ErrNotFound if the item doesn't exist.
func DeleteItem[T any, K ~string](
	ctx context.Context,
	region awsconfig.Region,
	tableName TableName,
	keyAttributeName KeyAttributeName,
	key K,
	opts ...dynamooptions.OptionDynamo,
) (*T, error) {
	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return nil, err
	}
	deleteItemInput := &dynamodb.DeleteItemInput{
		Key: map[string]types.AttributeValue{
			keyAttributeName.String(): &types.AttributeValueMemberS{Value: string(key)},
		},
		TableName:                   tableName.AWSString(),
		ReturnConsumedCapacity:      types.ReturnConsumedCapacityTotal,
		ReturnItemCollectionMetrics: types.ReturnItemCollectionMetricsSize,
		ReturnValues:                types.ReturnValueAllOld,
	}
	deletedItem, err := client.DeleteItem(ctx, deleteItemInput)
	if err != nil {
		return nil, err
	}
	if deletedItem.Attributes == nil {
		return nil, ErrNotFound
	}
	out := new(T)
	if err := attributevalue.UnmarshalMap(deletedItem.Attributes, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetItem Get the item in DynamoDB
//
// Type parameters:
//   - T: the type of the item to retrieve
//   - K: a string-compatible type for the key value
//
// Returns the retrieved item or ErrNotFound if the item doesn't exist.
func GetItem[T any, K ~string](
	ctx context.Context,
	region awsconfig.Region,
	tableName TableName,
	keyAttributeName KeyAttributeName,
	key K,
	opts ...dynamooptions.OptionDynamo,
) (*T, error) {
	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return nil, err
	}
	getItemInput := &dynamodb.GetItemInput{
		Key: map[string]types.AttributeValue{
			keyAttributeName.String(): &types.AttributeValueMemberS{Value: string(key)},
		},
		TableName: tableName.AWSString(),
		// https://docs.aws.amazon.com/ja_jp/amazondynamodb/latest/developerguide/HowItWorks.ReadConsistency.html
		ConsistentRead: aws.Bool(true),
	}
	getItem, err := client.GetItem(ctx, getItemInput)
	if err != nil {
		return nil, err
	}
	if getItem.Item == nil {
		return nil, ErrNotFound
	}
	out := new(T)
	if err := attributevalue.UnmarshalMap(getItem.Item, out); err != nil {
		return nil, err
	}
	return out, nil
}

// BatchGetItem Retrieve Dynamodb items in a batch process
// Note that the order of retrieval is not the order in which the keys are specified.
//
// Type parameters:
//   - T: the type of the item to retrieve
//   - K: a string-compatible type for the key value
func BatchGetItem[T any, K ~string](
	ctx context.Context,
	region awsconfig.Region,
	tableName TableName,
	keyAttributeName KeyAttributeName,
	keys []K,
	opts ...dynamooptions.OptionDynamo,
) ([]*T, error) {
	// DynamoDB allows a maximum batch size of 100 items.
	// https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_BatchGetItem.html
	const MaxBatchSize = 100

	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return nil, err
	}

	reqKeys := make([]map[string]types.AttributeValue, len(keys))
	for i, key := range keys {
		reqKeys[i] = map[string]types.AttributeValue{
			keyAttributeName.String(): &types.AttributeValueMemberS{Value: string(key)},
		}
	}

	resultItems := make([]*T, 0, len(keys))

	start := 0
	end := start + MaxBatchSize
	for start < len(reqKeys) {
		getReqs := make([]map[string]types.AttributeValue, 0, MaxBatchSize)
		if end > len(reqKeys) {
			end = len(reqKeys)
		}
		for _, v := range reqKeys[start:end] {
			getReqs = append(getReqs, v)
		}
		getItems, err := client.BatchGetItem(ctx, &dynamodb.BatchGetItemInput{
			RequestItems: map[string]types.KeysAndAttributes{
				tableName.String(): {Keys: getReqs},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("received batch error %+#v for batch getting. %w", getItems, err)
		}

		for _, v := range getItems.Responses[tableName.String()] {
			ret := new(T)
			if err := attributevalue.UnmarshalMap(v, ret); err != nil {
				return nil, fmt.Errorf("couldn't unmarshal item %+#v for batch getting. %w", v, err)
			}
			resultItems = append(resultItems, ret)
		}
		start = end
		end += MaxBatchSize
	}

	return resultItems, nil
}

// BatchWriteItem Write Dynamodb items in a batch process
// Type parameters:
//   - T: the type of the item to retrieve
func BatchWriteItem[T any](
	ctx context.Context,
	region awsconfig.Region,
	tableName TableName,
	items []T,
	opts ...dynamooptions.OptionDynamo,
) error {
	const (
		// MaxBatchSize DynamoDB allows a maximum batch size of 25 items.
		// https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_BatchWriteItem.html
		MaxBatchSize = 25
		// WriteWaitTime wait time between batch writes
		WriteWaitTime = 10 * time.Millisecond
	)

	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return err
	}

	start := 0
	end := start + MaxBatchSize
	for start < len(items) {
		writeReqs := make([]types.WriteRequest, 0, MaxBatchSize)
		if end > len(items) {
			end = len(items)
		}
		for _, v := range items[start:end] {
			item, err := attributevalue.MarshalMap(v)
			if err != nil {
				return fmt.Errorf("couldn't marshal item %+#v for batch writing. %w", v, err)
			} else {
				writeReqs = append(
					writeReqs,
					types.WriteRequest{PutRequest: &types.PutRequest{Item: item}},
				)
			}
		}
		if _, err := client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{tableName.String(): writeReqs},
		},
		); err != nil {
			return fmt.Errorf("received batch error %+#v for batch writing. %w", writeReqs, err)
		}
		if err := awstime.SleepWithContext(ctx, WriteWaitTime); err != nil {
			return err
		}
		start = end
		end += MaxBatchSize
	}

	return err
}
