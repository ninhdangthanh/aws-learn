output "aws_lb_listener_rule_tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-rule-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-1db3a3949ce205ad-002F-e7897d17984d2708_id" {
  value = "${aws_lb_listener_rule.tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-rule-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-1db3a3949ce205ad-002F-e7897d17984d2708.id}"
}

output "aws_lb_listener_tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-1db3a3949ce205ad_id" {
  value = "${aws_lb_listener.tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-1db3a3949ce205ad.id}"
}

output "aws_lb_listener_tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-9f103f6b181a6e5f_id" {
  value = "${aws_lb_listener.tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-9f103f6b181a6e5f.id}"
}

output "aws_lb_target_group_attachment_tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-targetgroup-002F-videocall-frontend-tg-002F-9422cf35f79d98ce-172-002E-31-002E-45-002E-10_id" {
  value = "${aws_lb_target_group_attachment.tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-targetgroup-002F-videocall-frontend-tg-002F-9422cf35f79d98ce-172-002E-31-002E-45-002E-10.id}"
}

output "aws_lb_target_group_tfer--videocall-frontend-tg_id" {
  value = "${aws_lb_target_group.tfer--videocall-frontend-tg.id}"
}

output "aws_lb_tfer--videocall-alb_id" {
  value = "${aws_lb.tfer--videocall-alb.id}"
}
