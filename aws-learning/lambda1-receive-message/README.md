# AWS Lambda: Receive Message

This project demonstrates a simple AWS Lambda function designed to receive and process messages. This could be triggered by various AWS services like SQS, SNS, or Kinesis.

## Flow of Action in AWS Console

1.  **IAM Role Creation**: Navigate to IAM > Roles and create a new role for the Lambda function with `AWSLambdaBasicExecutionRole` and permissions to consume messages from the triggering service (e.g., `AmazonSQSReadOnlyAccess` if using SQS).
2.  **Triggering Service Setup (e.g., SQS)**:
    *   **SQS Queue Creation**: Go to SQS > Queues and create a new Standard Queue. Configure its access policy to allow the Lambda's IAM role to receive messages.
    *   **Configure Lambda Trigger**: In the Lambda Console, select your function. Under "Add trigger," choose SQS (or your desired service), select the queue created, and configure the batch size and other settings. This establishes the connection so that messages in the queue will invoke your Lambda.
3.  **Lambda Function Creation**: Go to Lambda > Functions and create a new function. Choose Node.js runtime and associate it with the IAM role created in step 1. The `index.js` file contains the logic to process the incoming messages from the event object.
4.  **Testing**: Send a message to your SQS queue (or trigger your service). Monitor the Lambda function's CloudWatch logs to see the processing of the message. You can also use the "Test" button in the Lambda console to simulate an SQS event.
5.  **Monitoring**: In CloudWatch > Logs > Log groups, find the log group for your Lambda function to view its execution logs and verify message processing.