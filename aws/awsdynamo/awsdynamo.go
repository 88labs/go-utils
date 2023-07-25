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
func PutItem(ctx context.Context, region awsconfig.Region, tableName string, item any, opts ...dynamooptions.OptionDynamo) error {
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
		TableName: aws.String(tableName),
	}
	if _, err := client.PutItem(ctx, putItemInput); err != nil {
		return err
	}
	return nil
}

// UpdateItem Update the attributes of the item in DynamoDB Upsert if it does not exist
// expression: https://docs.aws.amazon.com/sdk-for-go/api/service/dynamodb/expression/#example_Builder_WithUpdate
func UpdateItem(
	ctx context.Context,
	region awsconfig.Region,
	tableName, keyFieldName, key string,
	update expression.UpdateBuilder,
	out any,
	opts ...dynamooptions.OptionDynamo,
) error {
	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return err
	}
	expr, err := expression.NewBuilder().WithUpdate(update).Build()
	if err != nil {
		return err
	}
	putItemInput := &dynamodb.UpdateItemInput{
		Key:                         map[string]types.AttributeValue{keyFieldName: &types.AttributeValueMemberS{Value: key}},
		TableName:                   aws.String(tableName),
		ExpressionAttributeNames:    expr.Names(),
		ExpressionAttributeValues:   expr.Values(),
		ReturnConsumedCapacity:      types.ReturnConsumedCapacityNone,
		ReturnItemCollectionMetrics: types.ReturnItemCollectionMetricsNone,
		ReturnValues:                types.ReturnValueAllNew,
		UpdateExpression:            expr.Update(),
	}
	updatedItem, err := client.UpdateItem(ctx, putItemInput)
	if err != nil {
		return err
	}
	if updatedItem.Attributes == nil {
		return ErrNotFound
	}
	if out != nil {
		if err := attributevalue.UnmarshalMap(updatedItem.Attributes, &out); err != nil {
			return err
		}
	}
	return nil
}

// DeleteItem Delete DynamoDB item
// expression: https://docs.aws.amazon.com/sdk-for-go/api/service/dynamodb/expression/#example_Builder_WithUpdate
// Mapping the retrieved item to `out`, must be a pointer to the `out`.
func DeleteItem(ctx context.Context, region awsconfig.Region, tableName, keyFieldName, key string, out any, opts ...dynamooptions.OptionDynamo) error {
	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return err
	}
	deleteItemInput := &dynamodb.DeleteItemInput{
		Key:                         map[string]types.AttributeValue{keyFieldName: &types.AttributeValueMemberS{Value: key}},
		TableName:                   aws.String(tableName),
		ReturnConsumedCapacity:      types.ReturnConsumedCapacityTotal,
		ReturnItemCollectionMetrics: types.ReturnItemCollectionMetricsSize,
		ReturnValues:                types.ReturnValueAllOld,
	}
	deletedItem, err := client.DeleteItem(ctx, deleteItemInput)
	if err != nil {
		return err
	}
	if deletedItem.Attributes == nil {
		return ErrNotFound
	}
	if out != nil {
		if err := attributevalue.UnmarshalMap(deletedItem.Attributes, &out); err != nil {
			return err
		}
	}
	return nil
}

// GetItem Get the item in DynamoDB
// Mapping the retrieved item to `out`, must be a pointer to the `out`.
func GetItem(ctx context.Context, region awsconfig.Region, tableName, keyFieldName, key string, out any, opts ...dynamooptions.OptionDynamo) error {
	c := dynamooptions.GetDynamoConf(opts...)
	client, err := GetClient(ctx, region, c.MaxAttempts, c.MaxBackoffDelay)
	if err != nil {
		return err
	}
	getItemInput := &dynamodb.GetItemInput{
		Key: map[string]types.AttributeValue{
			keyFieldName: &types.AttributeValueMemberS{Value: key},
		},
		TableName: aws.String(tableName),
		// https://docs.aws.amazon.com/ja_jp/amazondynamodb/latest/developerguide/HowItWorks.ReadConsistency.html
		ConsistentRead: aws.Bool(true),
	}
	getItem, err := client.GetItem(ctx, getItemInput)
	if err != nil {
		return err
	}
	if getItem.Item == nil {
		return ErrNotFound
	}
	if err := attributevalue.UnmarshalMap(getItem.Item, &out); err != nil {
		return err
	}
	return nil
}

// BatchGetItem Retrieve Dynamodb items in a batch process
// Return the retrieved item as a slice of type `T`.
// Note that the order of retrieval is not the order in which the keys are specified.
func BatchGetItem[T any, Key ~string](ctx context.Context, region awsconfig.Region, tableName, keyFieldName string, keys []Key, _ T, opts ...dynamooptions.OptionDynamo) ([]T, error) {
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
			keyFieldName: &types.AttributeValueMemberS{Value: string(key)},
		}
	}

	resultItems := make([]T, 0, len(keys))

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
				tableName: {Keys: getReqs},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("received batch error %+#v for batch getting. %v\n", getItems, err)
		}

		for _, v := range getItems.Responses[tableName] {
			var ret T
			if err := attributevalue.UnmarshalMap(v, &ret); err != nil {
				return nil, fmt.Errorf("Couldn't unmarshal item %+#v for batch getting. %v\n", v, err)
			}
			resultItems = append(resultItems, ret)
		}
		start = end
		end += MaxBatchSize
	}

	return resultItems, nil
}

// BatchWriteItem Write Dynamodb items in a batch process
func BatchWriteItem[T any](ctx context.Context, region awsconfig.Region, tableName string, items []T, opts ...dynamooptions.OptionDynamo) error {
	// DynamoDB allows a maximum batch size of 25 items.
	// https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_BatchWriteItem.html
	const MaxBatchSize = 25

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
				return fmt.Errorf("Couldn't marshal item %+#v for batch writing. %v\n", v, err)
			} else {
				writeReqs = append(
					writeReqs,
					types.WriteRequest{PutRequest: &types.PutRequest{Item: item}},
				)
			}
		}
		if _, err := client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{tableName: writeReqs},
		},
		); err != nil {
			return fmt.Errorf("received batch error %+#v for batch writing. %v\n", writeReqs, err)
		}
		if err := awstime.SleepWithContext(ctx, 10*time.Millisecond); err != nil {
			return err
		}
		start = end
		end += MaxBatchSize
	}

	return err
}
