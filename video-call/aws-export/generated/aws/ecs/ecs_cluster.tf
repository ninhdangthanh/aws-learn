resource "aws_ecs_cluster" "tfer--videocall-cluster" {
  name = "videocall-cluster"

  setting {
    name  = "containerInsights"
    value = "disabled"
  }
}
