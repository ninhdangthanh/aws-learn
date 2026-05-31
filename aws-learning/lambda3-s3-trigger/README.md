# AWS Lambda: S3 Trigger

This project sets up an AWS Lambda function that is automatically invoked when a new object is created in an S3 bucket. This is a common pattern for processing uploaded files.

## Flow of Action in AWS Console

1.  **IAM Role Creation**: Navigate to IAM > Roles and create a new role for the Lambda function. This role needs `AWSLambdaBasicExecutionRole` and permissions to read from the S3 bucket (e.g., `AmazonS3ReadOnlyAccess`).
2.  **S3 Bucket Creation**: Go to S3 > Buckets and create a new bucket. This will be the bucket that triggers the Lambda function. You can keep the default settings or customize as needed.
3.  **Lambda Function Creation**: Go to Lambda > Functions and create a new function. Choose Node.js runtime and associate it with the IAM role created in step 1. The `index.js` file contains the logic to process the S3 event, which includes details about the uploaded object.
4.  **Configure S3 Trigger**: In the Lambda Console, select your function. Under "Add trigger," choose S3. Select the S3 bucket created in step 2. Configure the event type (e.g., "All object create events"), and optionally specify a prefix or suffix to filter events (e.g., `.jpg` for image files). Acknowledge any warning about existing notifications.
5.  **Testing**: Upload a file to the configured S3 bucket (e.g., using the S3 console's "Upload" button). This action will trigger your Lambda function.
6.  **Monitoring**: In CloudWatch > Logs > Log groups, find the log group for your Lambda function. You should see logs indicating that the Lambda was invoked by the S3 event and processed the uploaded file information.