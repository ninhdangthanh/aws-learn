# CI/CD Pipeline for AWS Lambda

This project sets up a Continuous Integration and Continuous Deployment (CI/CD) pipeline for an AWS Lambda function using GitHub Actions and AWS services.

## Flow of Action in AWS Console

1.  **IAM Role Creation**: Navigate to IAM > Roles and create a new role for the Lambda function with permissions like `AWSLambdaBasicExecutionRole` and any other specific permissions your Lambda needs (e.g., S3 access, DynamoDB access).
2.  **S3 Bucket for Code Storage**: Create an S3 bucket to store your Lambda deployment packages. This bucket will be used by the CI/CD pipeline to upload the `zip` file of your Lambda code.
3.  **Lambda Function Creation**: Go to Lambda > Functions and create a new function. Choose Node.js runtime and associate it with the IAM role created in step 1. For initial deployment, you can use a sample "Hello World" code.
4.  **API Gateway (if applicable)**: If your Lambda is triggered by an HTTP request, navigate to API Gateway > APIs and create a new REST API or HTTP API. Configure a resource and method, then integrate it with your Lambda function.
5.  **GitHub Repository Secrets**: In your GitHub repository settings, add secrets for `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `S3_BUCKET_NAME`, and `LAMBDA_FUNCTION_NAME`. These are used by GitHub Actions to deploy your Lambda.
6.  **GitHub Actions Workflow**: The `.github/workflows/deploy-lambda-cicd.yml` file defines the CI/CD pipeline. This workflow will:
    *   Install dependencies.
    *   Lint and test the code (optional but recommended).
    *   Zip the Lambda function code.
    *   Upload the zipped file to the specified S3 bucket.
    *   Update the Lambda function code using the S3 object.
7.  **Monitor Deployment**: After a push to the main branch (or your configured branch), monitor the GitHub Actions workflow run. Once successful, check the Lambda function in the AWS Console to verify the updated code and version.