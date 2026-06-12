locals {
  oracle_network_config = jsonencode({
    awsvpcConfiguration = {
      subnets        = aws_subnet.private[*].id
      securityGroups = [aws_security_group.ecs_oracle.id]
      assignPublicIp = "DISABLED"
    }
  })
}

resource "aws_scheduler_schedule" "oracle_weekly" {
  name       = "wagr-${var.environment}-oracle-weekly"
  group_name = "default"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 30
  }

  schedule_expression          = var.oracle_weekly_schedule
  schedule_expression_timezone = "UTC"

  target {
    arn      = aws_ecs_cluster.main.arn
    role_arn = aws_iam_role.scheduler.arn

    ecs_parameters {
      task_definition_arn = aws_ecs_task_definition.oracle_weekly.arn
      launch_type         = "FARGATE"
      network_configuration {
        aws_vpc_configuration {
          subnets          = aws_subnet.private[*].id
          security_groups  = [aws_security_group.ecs_oracle.id]
          assign_public_ip = "DISABLED"
        }
      }
    }

    retry_policy {
      maximum_retry_attempts       = 2
      maximum_event_age_in_seconds = 3600
    }
  }
}

resource "aws_scheduler_schedule" "oracle_season" {
  name       = "wagr-${var.environment}-oracle-season"
  group_name = "default"

  flexible_time_window {
    mode                      = "FLEXIBLE"
    maximum_window_in_minutes = 60
  }

  schedule_expression          = var.oracle_season_schedule
  schedule_expression_timezone = "UTC"

  target {
    arn      = aws_ecs_cluster.main.arn
    role_arn = aws_iam_role.scheduler.arn

    ecs_parameters {
      task_definition_arn = aws_ecs_task_definition.oracle_season.arn
      launch_type         = "FARGATE"
      network_configuration {
        aws_vpc_configuration {
          subnets          = aws_subnet.private[*].id
          security_groups  = [aws_security_group.ecs_oracle.id]
          assign_public_ip = "DISABLED"
        }
      }
    }

    retry_policy {
      maximum_retry_attempts       = 2
      maximum_event_age_in_seconds = 7200
    }
  }
}
