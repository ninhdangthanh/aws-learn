resource "aws_iam_user_policy" "tfer--github-actions-videocall-deploy_name1" {
  name = "name1"

  policy = <<POLICY
{
  "Statement": [
    {
      "Action": "iam:PassRole",
      "Effect": "Allow",
      "Resource": "arn:aws:iam::677402243468:role/ecsTaskExecutionRole"
    }
  ],
  "Version": "2012-10-17"
}
POLICY

  user = "github-actions-videocall-deploy"
}
