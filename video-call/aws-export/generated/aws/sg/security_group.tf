resource "aws_security_group" "tfer--default_sg-0ab0d2965a9da066e" {
  description = "default VPC security group"

  egress {
    cidr_blocks = ["0.0.0.0/0"]
    from_port   = "0"
    protocol    = "-1"
    self        = "false"
    to_port     = "0"
  }

  ingress {
    from_port = "0"
    protocol  = "-1"
    self      = "true"
    to_port   = "0"
  }

  name   = "default"
  vpc_id = "vpc-0806d612b395af484"
}

resource "aws_security_group" "tfer--videocall-alb-sg_sg-02758c3c207fc2dfe" {
  description = "Security group for video call ALB"

  egress {
    cidr_blocks = ["0.0.0.0/0"]
    from_port   = "0"
    protocol    = "-1"
    self        = "false"
    to_port     = "0"
  }

  ingress {
    cidr_blocks = ["0.0.0.0/0"]
    from_port   = "443"
    protocol    = "tcp"
    self        = "false"
    to_port     = "443"
  }

  ingress {
    cidr_blocks = ["0.0.0.0/0"]
    from_port   = "80"
    protocol    = "tcp"
    self        = "false"
    to_port     = "80"
  }

  name   = "videocall-alb-sg"
  vpc_id = "vpc-0806d612b395af484"
}

resource "aws_security_group" "tfer--videocall-ecs-sg_sg-089599ea13047dc8d" {
  description = "Security group for video call ECS tasks"

  egress {
    cidr_blocks = ["0.0.0.0/0"]
    from_port   = "0"
    protocol    = "-1"
    self        = "false"
    to_port     = "0"
  }

  ingress {
    from_port       = "80"
    protocol        = "tcp"
    security_groups = ["${data.terraform_remote_state.sg.outputs.aws_security_group_tfer--videocall-alb-sg_sg-02758c3c207fc2dfe_id}"]
    self            = "false"
    to_port         = "80"
  }

  name   = "videocall-ecs-sg"
  vpc_id = "vpc-0806d612b395af484"
}
