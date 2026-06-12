resource "aws_ecs_cluster" "main" {
  name = "wagr-${var.environment}"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

resource "aws_cloudwatch_log_group" "gateway" {
  name              = "/wagr/${var.environment}/gateway"
  retention_in_days = 30
}

resource "aws_cloudwatch_log_group" "oracle" {
  name              = "/wagr/${var.environment}/oracle"
  retention_in_days = 14
}

locals {
  image_uri = "${aws_ecr_repository.wagr.repository_url}:${var.app_image_tag}"

  # Environment variables shared by both gateway and oracle tasks
  common_environment = [
    { name = "DB_HOST",    value = aws_db_instance.postgres.address },
    { name = "DB_PORT",    value = "5432" },
    { name = "DB_USER",    value = var.db_username },
    { name = "DB_NAME",    value = var.db_name },
    { name = "DB_SSLMODE", value = "require" },
    { name = "HEDERA_NETWORK",             value = var.hedera_network },
    { name = "HEDERA_USDC_TOKEN_ID",       value = var.hedera_usdc_token_id },
    { name = "HEDERA_ESCROW_CONTRACT_ID",  value = var.hedera_escrow_contract_id },
  ]

  # Secrets pulled from Secrets Manager at task start
  common_secrets = [
    { name = "DB_PASSWORD",        valueFrom = aws_secretsmanager_secret.db_password.arn },
    { name = "HEDERA_OPERATOR_ID", valueFrom = aws_secretsmanager_secret.hedera_operator_id.arn },
    { name = "HEDERA_OPERATOR_KEY",valueFrom = aws_secretsmanager_secret.hedera_operator_key.arn },
  ]
}

# --- Gateway Task Definition ---
resource "aws_ecs_task_definition" "gateway" {
  family                   = "wagr-${var.environment}-gateway"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.gateway_cpu
  memory                   = var.gateway_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name      = "gateway"
    image     = local.image_uri
    essential = true

    portMappings = [{ containerPort = 8080, protocol = "tcp" }]

    environment = concat(local.common_environment, [
      { name = "JWT_SECRET", value = "" }, # overridden by secret below
    ])

    secrets = concat(local.common_secrets, [
      { name = "JWT_SECRET", valueFrom = aws_secretsmanager_secret.jwt_secret.arn },
    ])

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.gateway.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "gateway"
      }
    }
  }])
}

# --- Oracle Weekly Task Definition ---
resource "aws_ecs_task_definition" "oracle_weekly" {
  family                   = "wagr-${var.environment}-oracle-weekly"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.oracle_cpu
  memory                   = var.oracle_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name      = "oracle"
    image     = local.image_uri
    essential = true
    command   = ["/oracle", "-job", "weekly"]

    environment = concat(local.common_environment, [
      { name = "ORACLE_JOB_TIMEOUT", value = var.oracle_job_timeout },
    ])

    secrets = local.common_secrets

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.oracle.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "oracle-weekly"
      }
    }
  }])
}

# --- Oracle Season-End Task Definition ---
resource "aws_ecs_task_definition" "oracle_season" {
  family                   = "wagr-${var.environment}-oracle-season"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.oracle_cpu
  memory                   = var.oracle_memory
  execution_role_arn       = aws_iam_role.ecs_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name      = "oracle"
    image     = local.image_uri
    essential = true
    command   = ["/oracle", "-job", "season"]

    environment = concat(local.common_environment, [
      { name = "ORACLE_JOB_TIMEOUT", value = var.oracle_job_timeout },
    ])

    secrets = local.common_secrets

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.oracle.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "oracle-season"
      }
    }
  }])
}

# --- Gateway ECS Service ---
resource "aws_ecs_service" "gateway" {
  name            = "wagr-${var.environment}-gateway"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.gateway.arn
  desired_count   = var.gateway_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.ecs_gateway.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.gateway.arn
    container_name   = "gateway"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.http]
}
