resource "aws_lb_listener" "tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-1db3a3949ce205ad" {
  certificate_arn = "arn:aws:acm:ap-southeast-1:677402243468:certificate/128babff-071f-46e1-8a5f-f4dd30371009"

  default_action {
    fixed_response {
      content_type = "text/plain"
      message_body = "Not Found"
      status_code  = "404"
    }

    order = "1"
    type  = "fixed-response"
  }

  load_balancer_arn = "${data.terraform_remote_state.alb.outputs.aws_lb_tfer--videocall-alb_id}"

  mutual_authentication {
    ignore_client_certificate_expiry = "false"
    mode                             = "off"
  }

  port                                 = "443"
  protocol                             = "HTTPS"
  routing_http_response_server_enabled = "true"
  ssl_policy                           = "ELBSecurityPolicy-TLS13-1-2-Res-PQ-2025-09"
}

resource "aws_lb_listener" "tfer--arn-003A-aws-003A-elasticloadbalancing-003A-ap-southeast-1-003A-677402243468-003A-listener-002F-app-002F-videocall-alb-002F-9615e9473ceed871-002F-9f103f6b181a6e5f" {
  default_action {
    order = "1"

    redirect {
      host        = "#{host}"
      path        = "/#{path}"
      port        = "443"
      protocol    = "HTTPS"
      query       = "#{query}"
      status_code = "HTTP_301"
    }

    type = "redirect"
  }

  load_balancer_arn                    = "${data.terraform_remote_state.alb.outputs.aws_lb_tfer--videocall-alb_id}"
  port                                 = "80"
  protocol                             = "HTTP"
  routing_http_response_server_enabled = "true"
}
