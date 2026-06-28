resource "aws_lb" "tfer--videocall-alb" {
  client_keep_alive = "3600"

  connection_logs {
    enabled = "false"
  }

  desync_mitigation_mode                      = "defensive"
  drop_invalid_header_fields                  = "false"
  enable_cross_zone_load_balancing            = "true"
  enable_deletion_protection                  = "false"
  enable_http2                                = "true"
  enable_tls_version_and_cipher_suite_headers = "false"
  enable_waf_fail_open                        = "false"
  enable_xff_client_port                      = "false"
  enable_zonal_shift                          = "false"
  idle_timeout                                = "60"
  internal                                    = "false"
  ip_address_type                             = "ipv4"
  load_balancer_type                          = "application"
  name                                        = "videocall-alb"
  preserve_host_header                        = "false"
  security_groups                             = ["sg-02758c3c207fc2dfe", "sg-0ab0d2965a9da066e"]

  subnet_mapping {
    subnet_id = "subnet-029dc17eaede50b97"
  }

  subnet_mapping {
    subnet_id = "subnet-0bd81f549904c4c0e"
  }

  subnets                    = ["subnet-029dc17eaede50b97", "subnet-0bd81f549904c4c0e"]
  xff_header_processing_mode = "append"
}
