resource "aws_lb_target_group_attachment" "tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-targetgroup-002F-videocall-frontend-tg-002F-9422cf35f79d98ce-172-002E-31-002E-45-002E-10" {
  target_group_arn = "arn:aws:elasticloadbalancing:ap-southeast-1:677402243468:targetgroup/videocall-frontend-tg/9422cf35f79d98ce"
  target_id        = "172.31.45.10"
}
