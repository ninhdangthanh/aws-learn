resource "aws_ecs_service" "tfer--videocall-cluster_videocall-service" {
  availability_zone_rebalancing = "ENABLED"
  cluster                       = "videocall-cluster"

  deployment_circuit_breaker {
    enable   = "true"
    rollback = "true"
  }

  deployment_controller {
    type = "ECS"
  }

  deployment_maximum_percent         = "200"
  deployment_minimum_healthy_percent = "100"
  desired_count                      = "1"
  enable_ecs_managed_tags            = "true"
  enable_execute_command             = "false"
  health_check_grace_period_seconds  = "0"
  launch_type                        = "FARGATE"

  load_balancer {
    container_name   = "frontend"
    container_port   = "80"
    target_group_arn = "arn:aws:elasticloadbalancing:ap-southeast-1:677402243468:targetgroup/videocall-frontend-tg/9422cf35f79d98ce"
  }

  name = "videocall-service"

  network_configuration {
    assign_public_ip = "true"
    security_groups  = ["sg-089599ea13047dc8d", "sg-0ab0d2965a9da066e"]
    subnets          = ["subnet-029dc17eaede50b97", "subnet-08980b50ca8a85c28", "subnet-0bd81f549904c4c0e"]
  }

  platform_version    = "1.4.0"
  scheduling_strategy = "REPLICA"
  task_definition     = "arn:aws:ecs:ap-southeast-1:677402243468:task-definition/videocall-task:8"
}
