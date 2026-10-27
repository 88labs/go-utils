#!/bin/sh
# Creates the SNS topic Debezium Server publishes to, the SQS queue the cdc
# consumer reads from, and the subscription between them.
set -e

EP="${AWS_ENDPOINT_URL:-http://floci:4566}"
TOPIC_NAME="${TOPIC_NAME:-cdc.cdc_source.orders}"
QUEUE_NAME="${QUEUE_NAME:-cdc-orders}"

echo "waiting for floci at ${EP}"
until aws sns list-topics --endpoint-url "${EP}" >/dev/null 2>&1; do
  sleep 1
done

TOPIC_ARN=$(aws sns create-topic --name "${TOPIC_NAME}" --endpoint-url "${EP}" --query TopicArn --output text)
# VisibilityTimeout stays above the consumer's HandlerTimeout (30s in the test).
QUEUE_URL=$(aws sqs create-queue --queue-name "${QUEUE_NAME}" --attributes VisibilityTimeout=60 --endpoint-url "${EP}" --query QueueUrl --output text)
QUEUE_ARN=$(aws sqs get-queue-attributes --queue-url "${QUEUE_URL}" --attribute-names QueueArn --endpoint-url "${EP}" --query Attributes.QueueArn --output text)

aws sns subscribe --topic-arn "${TOPIC_ARN}" --protocol sqs --notification-endpoint "${QUEUE_ARN}" --endpoint-url "${EP}" >/dev/null

echo "topic ${TOPIC_ARN} -> queue ${QUEUE_ARN}"
