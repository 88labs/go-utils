package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/kelseyhightower/envconfig"

	"github.com/88labs/andpad-approval-bff/auth/session"

	"github.com/aws/aws-sdk-go/aws/request"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"golang.org/x/oauth2"

	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/88labs/go-utils/cerrors"

	"github.com/aws/aws-dax-go/dax"
	aws_session "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/uuid"
)

type DBType string

const (
	DBTypeDynamoDB      DBType = "dynamodb"
	DBTypeDax           DBType = "dax"
	DBTypeDynamoDBLocal DBType = "dynamodb_local"
)

type DynamoDBConfig struct {
	// DB種別。dynamodb, dax, dynamodb_local
	DBType DBType `envconfig:"DYNAMO_DB_TYPE" default:"dynamodb_local"`
	// セッション管理に使うDynamoDBのリージョン
	Region string `envconfig:"DYNAMO_REGION" default:"ap-northeast-1"`
	// セッション管理に使うDynamoDB/DAXのエンドポイント
	// 実際のDynamo DBを使う場合は設定無視。DAX or local Dynamo DBのみ考慮。
	Endpoint string `envconfig:"DYNAMO_ENDPOINT" default:"http://localhost:8005"`
	// セッション管理用のTable
	SessionTable string `envconfig:"DYNAMO_SESSION_TABLE" default:"approval_session_local"`
}

// DynamoDBSessionRepository セッション管理のDynamoDB実装。
type DynamoDBSessionRepository struct {
	client itemClient
	config *DynamoDBConfig
}

type itemClient interface {
	GetItemWithContext(ctx aws.Context, input *dynamodb.GetItemInput, opts ...request.Option) (*dynamodb.GetItemOutput, error)
	PutItemWithContext(ctx aws.Context, input *dynamodb.PutItemInput, opts ...request.Option) (*dynamodb.PutItemOutput, error)
	DeleteItemWithContext(ctx aws.Context, input *dynamodb.DeleteItemInput, opts ...request.Option) (*dynamodb.DeleteItemOutput, error)
}

func (m *DynamoDBSessionRepository) GetSession(ctx context.Context, id string) (*session.Session, error) {
	span, ctx := tracer.StartSpanFromContext(ctx, "GetSession")
	defer span.Finish()

	input := &dynamodb.GetItemInput{
		TableName: aws.String(m.config.SessionTable),
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {S: &id},
		},
	}

	output, err := m.client.GetItemWithContext(ctx, input)
	if err != nil {
		return nil, cerrors.New(cerrors.UnknownErr, err, "")
	}

	if output.Item == nil {
		return nil, cerrors.New(cerrors.NotFoundErr, nil, "")
	}

	userId, err := strconv.Atoi(*output.Item["UserId"].N)
	if err != nil {
		// PreconditionErrの方が良い気はする
		return nil, cerrors.New(cerrors.UnknownErr, err, "UserIdの形式が不正です")
	}

	clientId, err := strconv.Atoi(*output.Item["ClientId"].N)
	if err != nil {
		// PreconditionErrの方が良い気はする
		return nil, cerrors.New(cerrors.UnknownErr, err, "clientIdの形式が不正です")
	}

	expiredAt, err := strconv.ParseInt(*output.Item["ExpiredAt"].N, 10, 64)
	if expiredAt < time.Now().Unix() {
		return nil, cerrors.New(cerrors.NotFoundErr, err, "セッションの有効期限切れ")
	}

	tokenExpiredAt, err := strconv.ParseInt(*output.Item["TokenExpiredAt"].N, 10, 64)
	if err != nil {
		// PreconditionErrの方が良い気はする
		return nil, cerrors.New(cerrors.UnknownErr, err, "clientIdの形式が不正です")
	}

	return session.FromRawSession(
		*output.Item["Id"].S,
		int32(userId),
		int32(clientId),
		&oauth2.Token{
			AccessToken:  *output.Item["AccessToken"].S,
			RefreshToken: *output.Item["RefreshToken"].S,
			Expiry:       time.Unix(tokenExpiredAt, 0),
		},
		expiredAt,
	), nil
}

func (m *DynamoDBSessionRepository) CreateSession(ctx context.Context, s *session.Session) (string, error) {
	id := uuid.NewString()
	userIdStr := strconv.Itoa(int(s.UserId()))
	clientIdStr := strconv.Itoa(int(s.ClientId()))
	tokenExpiredAt := strconv.FormatInt(s.Token().Expiry.Unix(), 10)
	expiredAt := strconv.FormatInt(s.ExpiredAt(), 10)

	input := &dynamodb.PutItemInput{
		TableName: aws.String(m.config.SessionTable),
		Item: map[string]*dynamodb.AttributeValue{
			"Id":             {S: &id},
			"UserId":         {N: &userIdStr},
			"ClientId":       {N: &clientIdStr},
			"AccessToken":    {S: &s.Token().AccessToken},
			"RefreshToken":   {S: &s.Token().RefreshToken},
			"TokenExpiredAt": {N: &tokenExpiredAt},
			"ExpiredAt":      {N: &expiredAt},
		},
	}

	_, err := m.client.PutItemWithContext(ctx, input)
	if err != nil {
		return "", cerrors.New(cerrors.UnknownErr, err, "")
	}

	return id, nil
}

func (m *DynamoDBSessionRepository) DeleteSession(ctx context.Context, id string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(m.config.SessionTable),
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {S: &id},
		},
	}

	if _, err := m.client.DeleteItemWithContext(ctx, input); err == nil {
		return nil
	} else {
		return cerrors.New(cerrors.UnknownErr, err, "セッションの削除ができません")
	}
}

func dynamoDBClient(config *DynamoDBConfig) itemClient {
	var (
		cred     *credentials.Credentials
		endpoint *string
	)

	if config.DBType == DBTypeDynamoDBLocal {
		cred = credentials.NewStaticCredentials("dummy", "dummy", "dummy")
		endpoint = aws.String(config.Endpoint)
	}

	awsConfig := &aws.Config{
		Region:      aws.String(config.Region),
		Endpoint:    endpoint,
		DisableSSL:  aws.Bool(false),
		Credentials: cred,
	}

	awsSession, err := aws_session.NewSession(awsConfig)
	if err != nil {
		panic(err)
	}

	return dynamodb.New(awsSession)
}

func daxClient(config *DynamoDBConfig) itemClient {
	awsConfig := &aws.Config{
		Region: aws.String(config.Region),
	}
	awsSession, err := aws_session.NewSession(awsConfig)
	if err != nil {
		panic(err)
	}
	secureCfg := dax.NewConfigWithSession(*awsSession)
	secureCfg.HostPorts = []string{config.Endpoint}

	client, err := dax.New(secureCfg)
	if err != nil {
		panic(err)
	}

	return client
}

func NewDynamoDBSessionRepository(config DynamoDBConfig) *DynamoDBSessionRepository {
	var client itemClient
	switch config.DBType {
	case DBTypeDax:
		client = daxClient(&config)
	case DBTypeDynamoDB, DBTypeDynamoDBLocal:
		client = dynamoDBClient(&config)
	default:
		panic(fmt.Errorf("unknown db type %v. %v, %v or %v is supported", config.DBType, DBTypeDynamoDB, DBTypeDax, DBTypeDynamoDBLocal))
	}

	return &DynamoDBSessionRepository{
		client: client,
		config: &config,
	}
}

func LoadSessionDBConfig(prefix string) *DynamoDBConfig {
	c := &DynamoDBConfig{}
	if err := envconfig.Process(prefix, c); err != nil {
		panic(err)
	}
	return c
}
