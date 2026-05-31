# AWS Lambda: Call Lambda 1

This project features an AWS Lambda function (`lambda2-call-lambda1`) that invokes another Lambda function (`lambda1-receive-message`). This demonstrates inter-service communication within AWS Lambda.

## Flow of Action in AWS Console

1.  **Ensure Lambda1 is Deployed**: Make sure the `lambda1-receive-message` function is already deployed and configured as described in its `README.md`.
2.  **IAM Role Creation for Lambda2**: Navigate to IAM > Roles and create a new role for `lambda2-call-lambda1`. This role needs `AWSLambdaBasicExecutionRole` and critically, `lambda:InvokeFunction` permissions for `lambda1-receive-message`. You can achieve this by adding an inline policy or attaching a managed policy like `AWSLambdaRole` and then refining its permissions.
3.  **Lambda Function Creation (Lambda2)**: Go to Lambda > Functions and create a new function (e.g., `lambda2-call-lambda1`). Choose Node.js runtime and associate it with the IAM role created in step 2. The `index.js` file contains the logic to invoke `lambda1-receive-message` using the AWS SDK.
4.  **Configure Lambda2 Environment Variables**: In the Lambda2 configuration, under "Environment variables," add a variable (e.g., `TARGET_LAMBDA_NAME`) with the value of the ARN or name of your `lambda1-receive-message` function.
5.  **Testing Lambda2**: Use the "Test" button in the Lambda2 console. Configure a test event (e.g., a simple JSON object). When you run the test, `lambda2-call-lambda1` will execute and attempt to invoke `lambda1-receive-message`.
6.  **Monitoring**: Check the CloudWatch logs for both `lambda2-call-lambda1` and `lambda1-receive-message` to verify that the invocation was successful and that `lambda1-receive-message` processed the payload as expected.