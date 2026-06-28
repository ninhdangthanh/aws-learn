resource "aws_lb_listener_rule" "tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-rule-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-1db3a3949ce205ad-002F-e7897d17984d2708" {
  action {
    forward {
      stickiness {
        duration = "3600"
        enabled  = "false"
      }

      target_group {
        arn    = "arn:aws:elasticloadbalancing:ap-southeast-1:677402243468:targetgroup/videocall-frontend-tg/9422cf35f79d98ce"
        weight = "1"
      }
    }

    order            = "1"
    target_group_arn = "arn:aws:elasticloadbalancing:ap-southeast-1:677402243468:targetgroup/videocall-frontend-tg/9422cf35f79d98ce"
    type             = "forward"
  }

  condition {
    host_header {
      values = ["ninh-video-call-demo.food"]
    }
  }

  listener_arn = "${data.terraform_remote_state.alb.outputs.aws_lb_listener_tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-1db3a3949ce205ad_id}"
  priority     = "1"
}
