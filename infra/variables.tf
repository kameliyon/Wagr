variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "app_image_tag" {
  description = "Docker image tag to deploy (e.g. git SHA)"
  type        = string
  default     = "latest"
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

# --- ECS Gateway ---
variable "gateway_cpu" {
  type    = number
  default = 512
}

variable "gateway_memory" {
  type    = number
  default = 1024
}

variable "gateway_desired_count" {
  description = "Number of gateway tasks to run"
  type        = number
  default     = 1
}

# --- ECS Oracle ---
variable "oracle_cpu" {
  type    = number
  default = 256
}

variable "oracle_memory" {
  type    = number
  default = 512
}

variable "oracle_job_timeout" {
  description = "Max runtime for oracle jobs (Go duration string, e.g. '10m')"
  type        = string
  default     = "10m"
}

variable "oracle_weekly_schedule" {
  description = "EventBridge cron for the weekly payout job (UTC)"
  type        = string
  default     = "cron(0 9 ? * TUE *)"
}

variable "oracle_season_schedule" {
  description = "EventBridge cron for the season-end payout job (UTC)"
  type        = string
  default     = "cron(0 9 ? * WED *)"
}

# --- RDS ---
variable "db_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "db_multi_az" {
  description = "Enable Multi-AZ for RDS (recommended for prod)"
  type        = bool
  default     = false
}

variable "db_name" {
  type    = string
  default = "wagr"
}

variable "db_username" {
  type    = string
  default = "wagr"
}

# --- Hedera ---
variable "hedera_network" {
  type    = string
  default = "testnet"
}

variable "hedera_usdc_token_id" {
  type    = string
  default = ""
}

variable "hedera_escrow_contract_id" {
  type    = string
  default = ""
}
