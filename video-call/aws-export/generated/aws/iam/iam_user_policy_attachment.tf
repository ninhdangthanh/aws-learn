resource "aws_iam_user_policy_attachment" "tfer--github-actions-videocall-deploy_AmazonEC2ContainerRegistryPowerUser" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryPowerUser"
  user       = "github-actions-videocall-deploy"
}

resource "aws_iam_user_policy_attachment" "tfer--github-actions-videocall-deploy_AmazonECS_FullAccess" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonECS_FullAccess"
  user       = "github-actions-videocall-deploy"
}

resource "aws_iam_user_policy_attachment" "tfer--github-actions-videocall-deploy_AmazonSSMReadOnlyAccess" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMReadOnlyAccess"
  user       = "github-actions-videocall-deploy"
}

resource "aws_iam_user_policy_attachment" "tfer--github-actions-videocall-deploy_CloudWatchLogsFullAccess" {
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchLogsFullAccess"
  user       = "github-actions-videocall-deploy"
}

resource "aws_iam_user_policy_attachment" "tfer--github-actions-videocall-deploy_IAMReadOnlyAccess" {
  policy_arn = "arn:aws:iam::aws:policy/IAMReadOnlyAccess"
  user       = "github-actions-videocall-deploy"
}

resource "aws_iam_user_policy_attachment" "tfer--ninh-mac-air-m4_AWSAuditManagerAdministratorAccess" {
  policy_arn = "arn:aws:iam::aws:policy/AWSAuditManagerAdministratorAccess"
  user       = "ninh-mac-air-m4"
}

resource "aws_iam_user_policy_attachment" "tfer--ninh-mac-air-m4_AWSManagementConsoleAdministratorAccess" {
  policy_arn = "arn:aws:iam::aws:policy/job-function/AWSManagementConsoleAdministratorAccess"
  user       = "ninh-mac-air-m4"
}

resource "aws_iam_user_policy_attachment" "tfer--ninh-mac-air-m4_AdministratorAccess" {
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess"
  user       = "ninh-mac-air-m4"
}

resource "aws_iam_user_policy_attachment" "tfer--ninh-mac-air-m4_AdministratorAccess-AWSElasticBeanstalk" {
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess-AWSElasticBeanstalk"
  user       = "ninh-mac-air-m4"
}

resource "aws_iam_user_policy_attachment" "tfer--ninh-mac-air-m4_AdministratorAccess-Amplify" {
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess-Amplify"
  user       = "ninh-mac-air-m4"
}
