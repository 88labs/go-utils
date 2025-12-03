package awsdynamo

import "github.com/aws/aws-sdk-go-v2/aws"

type TableName string

func (t TableName) String() string {
	return string(t)
}

func (t TableName) AWSString() *string {
	return aws.String(string(t))
}

type KeyAttributeName string

func (k KeyAttributeName) String() string {
	return string(k)
}

func (k KeyAttributeName) AWSString() *string {
	return aws.String(string(k))
}
