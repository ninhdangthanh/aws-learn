# AWS Lambda: EventBridge Time Trigger

This project demonstrates an AWS Lambda function that is invoked on a regular schedule using Amazon EventBridge (formerly CloudWatch Events). This is useful for cron-like jobs, periodic tasks, or scheduled data processing.

## Flow of Action in AWS Console

1.  **IAM Role Creation**: Navigate to IAM > Roles and create a new role for the Lambda function with `AWSLambdaBasicExecutionRole`.
2.  **Lambda Function Creation**: Go to Lambda > Functions and create a new function (e.g., `lambda5-eventbridge-time`). Choose Node.js runtime and associate it with the IAM role created in step 1. The `index.js` file contains the logic that will be executed on a schedule.
3.  **EventBridge Rule Configuration**: In the Lambda Console, select your function. Under "Add trigger," choose "EventBridge (CloudWatch Events)." Create a new rule.
    *   **Rule Type**: Select "Schedule." 
    *   **Schedule Expression**: Define your desired schedule using either a fixed rate (e.g., `rate(5 minutes)`) or a cron expression (e.g., `cron(0 10 * * ? *)` for 10 AM UTC daily). 
    *   **Rule Name**: Give your rule a descriptive name.
4.  **Testing**: Wait for the scheduled time or manually trigger the EventBridge rule for immediate testing (though direct manual trigger of a schedule rule is not straightforward; often, it's easier to briefly change the cron to a `rate(1 minute)` for testing, then change it back).
5.  **Monitoring**: In CloudWatch > Logs > Log groups, find the log group for your Lambda function. You should see logs indicating that the Lambda was invoked at the scheduled times by the EventBridge rule.