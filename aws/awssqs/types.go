package awssqs

import "github.com/aws/aws-sdk-go-v2/aws"

type QueueURL string

func (q QueueURL) String() string {
	return string(q)
}

func (q QueueURL) AWSString() *string {
	return aws.String(string(q))
}
