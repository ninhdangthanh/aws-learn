resource "aws_iam_role_policy" "tfer--ecsTaskExecutionRole_name2" {
  name = "name2"

  policy = <<POLICY
{
  "Statement": [
    {
      "Action": [
        "ssm:GetParameters",
        "ssm:GetParameter"
      ],
      "Effect": "Allow",
      "Resource": "arn:aws:ssm:ap-southeast-1:677402243468:parameter/videocall/*",
      "Sid": "ReadVideoCallParameters"
    }
  ],
  "Version": "2012-10-17"
}
POLICY

  role = "ecsTaskExecutionRole"
}
