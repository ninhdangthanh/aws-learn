# AWS Lambda: API Hello

This project creates an AWS Lambda function exposed via API Gateway, providing a simple "Hello World" API endpoint. This is a fundamental setup for serverless HTTP APIs.

## Flow of Action in AWS Console

1.  **IAM Role Creation**: Navigate to IAM > Roles and create a new role for the Lambda function with `AWSLambdaBasicExecutionRole`.
2.  **Lambda Function Creation**: Go to Lambda > Functions and create a new function (e.g., `lambda4-api-hello`). Choose Node.js runtime and associate it with the IAM role created in step 1. The `index.js` file contains the logic to respond to HTTP requests.
3.  **API Gateway Configuration**: In the Lambda Console, select your function. Under "Add trigger," choose "API Gateway." Create a new API, select "REST API" or "HTTP API," set "Security" to "Open," and choose a deployment stage (e.g., `dev`). This will automatically create an API Gateway endpoint and integrate it with your Lambda function.
4.  **Testing the API**: After the API Gateway is created, a URL will be displayed on the Lambda function's "Triggers" section. Open this URL in a web browser or use a tool like `curl` to send an HTTP request. You should receive the "Hello from Lambda!" response.
5.  **Monitoring**: In CloudWatch > Logs > Log groups, find the log group for your Lambda function. You can also view API Gateway logs (if enabled) in CloudWatch to monitor incoming requests and responses.