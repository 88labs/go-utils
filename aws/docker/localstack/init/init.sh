#!/usr/bin/env bash
set -x
awslocal s3 mb s3://test
awslocal sqs create-queue --queue-name test-queue
set +x
