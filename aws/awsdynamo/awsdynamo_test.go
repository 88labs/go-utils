package awsdynamo_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"

	"github.com/88labs/go-utils/ulid"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awsdynamo"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

const (
	TestTable           = "test"
	TestDynamoEndpoint  = "http://127.0.0.1:28002" // use local dynamo
	TestRegion          = awsconfig.RegionTokyo
	TestAccessKey       = "DUMMYACCESSKEYEXAMPLE"
	TestSecretAccessKey = "DUMMYSECRETKEYEXAMPLE"
)

type Test struct {
	ID        string                  `json:"id" dynamodbav:"id"`
	Name      string                  `json:"name" dynamodbav:"name"`
	CreatedAt attributevalue.UnixTime `json:"created_at" dynamodbav:"created_at"`
}

func TestPutItem(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithDynamoEndpoint(TestDynamoEndpoint),
		ctxawslocal.WithAccessKey(TestAccessKey),
		ctxawslocal.WithSecretAccessKey(TestSecretAccessKey),
	)

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		item := Test{
			ID:        ulid.MustNew().String(),
			Name:      faker.Name(),
			CreatedAt: attributevalue.UnixTime(time.Now()),
		}
		err := awsdynamo.PutItem(ctx, TestRegion, TestTable, item)
		assert.NoError(t, err)
	})
	t.Run("New/Update", func(t *testing.T) {
		t.Parallel()
		item := Test{
			ID:        ulid.MustNew().String(),
			Name:      faker.Name(),
			CreatedAt: attributevalue.UnixTime(time.Now()),
		}
		err := awsdynamo.PutItem(ctx, TestRegion, TestTable, item)
		assert.NoError(t, err)
		item.Name = faker.Name()
		err = awsdynamo.PutItem(ctx, TestRegion, TestTable, item)
		assert.NoError(t, err)
	})
}

func TestGetItem(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithDynamoEndpoint(TestDynamoEndpoint),
		ctxawslocal.WithAccessKey(TestAccessKey),
		ctxawslocal.WithSecretAccessKey(TestSecretAccessKey),
	)
	testItem := Test{
		ID:        ulid.MustNew().String(),
		Name:      faker.Name(),
		CreatedAt: attributevalue.UnixTime(time.Now()),
	}
	err := awsdynamo.PutItem(ctx, TestRegion, TestTable, testItem)
	assert.NoError(t, err)

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		out, err := awsdynamo.GetItem[Test](ctx, TestRegion, TestTable, "id", testItem.ID)
		assert.NoError(t, err)
		assert.Equal(t, testItem.ID, out.ID)
		assert.Equal(t, testItem.Name, out.Name)
		expectedCreatedAt, err := testItem.CreatedAt.MarshalDynamoDBAttributeValue()
		assert.NoError(t, err)
		actualCreatedAt, err := out.CreatedAt.MarshalDynamoDBAttributeValue()
		assert.NoError(t, err)
		assert.Equal(t, expectedCreatedAt, actualCreatedAt)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		_, err := awsdynamo.GetItem[Test](ctx, TestRegion, TestTable, "id", "NOT_FOUND")
		assert.Error(t, err)
		assert.ErrorIs(t, awsdynamo.ErrNotFound, err)
	})
}

func TestDeleteItem(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithDynamoEndpoint(TestDynamoEndpoint),
		ctxawslocal.WithAccessKey(TestAccessKey),
		ctxawslocal.WithSecretAccessKey(TestSecretAccessKey),
	)

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		testItem := Test{
			ID:        ulid.MustNew().String(),
			Name:      faker.Name(),
			CreatedAt: attributevalue.UnixTime(time.Now()),
		}
		err := awsdynamo.PutItem(ctx, TestRegion, TestTable, testItem)
		assert.NoError(t, err)

		out, err := awsdynamo.DeleteItem[Test](ctx, TestRegion, TestTable, "id", testItem.ID)
		assert.NoError(t, err)
		assert.Equal(t, testItem.ID, out.ID)
		assert.Equal(t, testItem.Name, out.Name)
		expectedCreatedAt, err := testItem.CreatedAt.MarshalDynamoDBAttributeValue()
		assert.NoError(t, err)
		actualCreatedAt, err := out.CreatedAt.MarshalDynamoDBAttributeValue()
		assert.NoError(t, err)
		assert.Equal(t, expectedCreatedAt, actualCreatedAt)
	})

	t.Run("Delete NotFound", func(t *testing.T) {
		t.Parallel()
		_, err := awsdynamo.DeleteItem[Test](ctx, TestRegion, TestTable, "id", "NOT_FOUND")
		assert.Error(t, err)
		assert.ErrorIs(t, awsdynamo.ErrNotFound, err)
	})
}

func TestUpdateItem(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithDynamoEndpoint(TestDynamoEndpoint),
		ctxawslocal.WithAccessKey(TestAccessKey),
		ctxawslocal.WithSecretAccessKey(TestSecretAccessKey),
	)

	t.Run("Update", func(t *testing.T) {
		t.Parallel()
		testItem := Test{
			ID:        ulid.MustNew().String(),
			Name:      faker.Name(),
			CreatedAt: attributevalue.UnixTime(time.Now()),
		}
		err := awsdynamo.PutItem(ctx, TestRegion, TestTable, testItem)
		assert.NoError(t, err)

		updateName := faker.Name()
		update := expression.Set(
			expression.Name("name"),
			expression.Value(updateName),
		)
		out, err := awsdynamo.UpdateItem[Test](ctx, TestRegion, TestTable, "id", testItem.ID, update)
		assert.NoError(t, err)
		assert.Equal(t, testItem.ID, out.ID)
		assert.Equal(t, updateName, out.Name)
		expectedCreatedAt, err := testItem.CreatedAt.MarshalDynamoDBAttributeValue()
		assert.NoError(t, err)
		actualCreatedAt, err := out.CreatedAt.MarshalDynamoDBAttributeValue()
		assert.NoError(t, err)
		assert.Equal(t, expectedCreatedAt, actualCreatedAt)
	})
}

func TestBatchGetItem(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithDynamoEndpoint(TestDynamoEndpoint),
		ctxawslocal.WithAccessKey(TestAccessKey),
		ctxawslocal.WithSecretAccessKey(TestSecretAccessKey),
	)

	makeItems := func(size int) ([]string, []Test) {
		ids := make([]string, 0, size)
		testItems := make([]Test, 0, size)
		for i := 0; i < size; i++ {
			item := Test{
				ID:        ulid.MustNew().String(),
				Name:      faker.Name(),
				CreatedAt: attributevalue.UnixTime(time.Now()),
			}
			err := awsdynamo.PutItem(ctx, TestRegion, TestTable, item)
			assert.NoError(t, err)
			testItems = append(testItems, item)
			ids = append(ids, item.ID)
		}
		return ids, testItems
	}

	t.Run("Get 101 items", func(t *testing.T) {
		t.Parallel()
		ids, testItems := makeItems(101)
		out, err := awsdynamo.BatchGetItem[Test](ctx, TestRegion, TestTable, "id", ids)
		assert.NoError(t, err)
		sort.Slice(testItems, func(i, j int) bool {
			return testItems[i].ID < testItems[j].ID
		})
		sort.Slice(out, func(i, j int) bool {
			return out[i].ID < out[j].ID
		})
		for i, testItem := range testItems {
			assert.Equal(t, testItem.ID, out[i].ID)
			assert.Equal(t, testItem.Name, out[i].Name)

			expectedCreatedAt, err := testItem.CreatedAt.MarshalDynamoDBAttributeValue()
			assert.NoError(t, err)
			actualCreatedAt, err := out[i].CreatedAt.MarshalDynamoDBAttributeValue()
			assert.NoError(t, err)
			assert.Equal(t, expectedCreatedAt, actualCreatedAt)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		out, err := awsdynamo.BatchGetItem[Test](ctx, TestRegion, TestTable, "id", []string{"NOT_FOUND"})
		assert.NoError(t, err)
		assert.Len(t, out, 0)
	})
}

func TestBatchWriteItem(t *testing.T) {
	t.Parallel()
	ctx := ctxawslocal.WithContext(
		context.Background(),
		ctxawslocal.WithDynamoEndpoint(TestDynamoEndpoint),
		ctxawslocal.WithAccessKey(TestAccessKey),
		ctxawslocal.WithSecretAccessKey(TestSecretAccessKey),
	)

	makeItems := func(size int) ([]string, []Test) {
		ids := make([]string, 0, size)
		testItems := make([]Test, 0, size)
		for i := 0; i < size; i++ {
			item := Test{
				ID:        ulid.MustNew().String(),
				Name:      faker.Name(),
				CreatedAt: attributevalue.UnixTime(time.Now()),
			}
			testItems = append(testItems, item)
			ids = append(ids, item.ID)
		}
		return ids, testItems
	}

	t.Run("Write 26 items", func(t *testing.T) {
		t.Parallel()
		ids, testItems := makeItems(26)
		err := awsdynamo.BatchWriteItem(ctx, TestRegion, TestTable, testItems)
		assert.NoError(t, err)

		out, err := awsdynamo.BatchGetItem[Test](ctx, TestRegion, TestTable, "id", ids)
		assert.NoError(t, err)
		assert.Equal(t, len(testItems), len(out))
	})
}
