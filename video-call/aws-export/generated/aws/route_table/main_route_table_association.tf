resource "aws_main_route_table_association" "tfer--vpc-0806d612b395af484" {
  route_table_id = "${data.terraform_remote_state.route_table.outputs.aws_route_table_tfer--rtb-0e1a6978d24037411_id}"
  vpc_id         = "${data.terraform_remote_state.vpc.outputs.aws_vpc_tfer--vpc-0806d612b395af484_id}"
}
