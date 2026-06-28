resource "aws_route_table" "tfer--rtb-0e1a6978d24037411" {
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = "igw-05891885242b3b6c1"
  }

  vpc_id = "${data.terraform_remote_state.vpc.outputs.aws_vpc_tfer--vpc-0806d612b395af484_id}"
}
