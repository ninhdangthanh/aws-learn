resource "aws_internet_gateway" "tfer--igw-05891885242b3b6c1" {
  vpc_id = "${data.terraform_remote_state.vpc.outputs.aws_vpc_tfer--vpc-0806d612b395af484_id}"
}
