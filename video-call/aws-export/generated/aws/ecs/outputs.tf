output "aws_ecs_cluster_tfer--videocall-cluster_id" {
  value = "${aws_ecs_cluster.tfer--videocall-cluster.id}"
}

output "aws_ecs_service_tfer--videocall-cluster_videocall-service_id" {
  value = "${aws_ecs_service.tfer--videocall-cluster_videocall-service.id}"
}

output "aws_ecs_task_definition_tfer--task-definition-002F-videocall-task_id" {
  value = "${aws_ecs_task_definition.tfer--task-definition-002F-videocall-task.id}"
}
