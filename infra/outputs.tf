output "alb_dns_name" {
  description = "Public DNS name of the load balancer — point your domain here"
  value       = aws_lb.main.dns_name
}

output "ecr_repository_url" {
  description = "ECR URL for docker push/pull"
  value       = aws_ecr_repository.wagr.repository_url
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "rds_endpoint" {
  description = "RDS host (used as DB_HOST)"
  value       = aws_db_instance.postgres.address
}

output "push_commands" {
  description = "Commands to build and push the Docker image"
  value       = <<-EOT
    aws ecr get-login-password --region ${var.aws_region} | \
      docker login --username AWS --password-stdin ${aws_ecr_repository.wagr.repository_url}

    docker build -t wagr .
    docker tag wagr:latest ${aws_ecr_repository.wagr.repository_url}:latest
    docker push ${aws_ecr_repository.wagr.repository_url}:latest
  EOT
}
