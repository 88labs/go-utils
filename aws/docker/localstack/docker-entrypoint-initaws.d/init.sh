#!/bin/bash
echo "init setup start!"
awslocal s3 mb s3://hoget
awslocal sqs create-queue --queue-name test-queue
echo "init setup done!"
