# ── Shared env var blocks ──────────────────────────────────────────────────────

locals {
  db_env = {
    DATABASE_URL = { value = var.database_url, is_secret = true }
  }

  hedera_env = {
    HEDERA_NETWORK            = { value = var.hedera_network }
    HEDERA_USDC_TOKEN_ID      = { value = var.hedera_usdc_token_id }
    HEDERA_ESCROW_CONTRACT_ID = { value = var.hedera_escrow_contract_id }
    HEDERA_OPERATOR_ID        = { value = var.hedera_operator_id, is_secret = true }
    HEDERA_OPERATOR_KEY       = { value = var.hedera_operator_key, is_secret = true }
  }

  oracle_env = merge(local.db_env, local.hedera_env, {
    ORACLE_JOB_TIMEOUT = { value = var.oracle_job_timeout }
  })
}

# ── API Gateway web service ────────────────────────────────────────────────────

resource "render_web_service" "gateway" {
  name          = "wagr-gateway-${var.environment}"
  plan          = "starter"
  region        = var.render_region
  start_command = "/gateway"

  runtime_source = {
    docker = {
      auto_deploy = true
      branch      = "master"
      repo_url    = var.github_repo_url
    }
  }

  env_vars = merge(local.db_env, local.hedera_env, {
    JWT_SECRET           = { value = var.jwt_secret, is_secret = true }
    CORS_ALLOWED_ORIGINS = { value = "https://wagrs.app,https://www.wagrs.app" }
    PORT                 = { value = "8080" }
  })

  health_check_path = "/health"
  custom_domains    = [{ name = "api.wagrs.app" }]
}

# ── Oracle cron jobs ───────────────────────────────────────────────────────────

resource "render_cron_job" "oracle_weekly" {
  name          = "wagr-oracle-weekly-${var.environment}"
  plan          = "starter"
  region        = var.render_region
  schedule      = "0 9 * * 2"
  start_command = "/oracle -job=weekly"

  runtime_source = {
    docker = {
      auto_deploy = true
      branch      = "master"
      repo_url    = var.github_repo_url
    }
  }

  env_vars = local.oracle_env
}

resource "render_cron_job" "oracle_season" {
  name          = "wagr-oracle-season-${var.environment}"
  plan          = "starter"
  region        = var.render_region
  schedule      = "0 12 * * 1"
  start_command = "/oracle -job=season"

  runtime_source = {
    docker = {
      auto_deploy = true
      branch      = "master"
      repo_url    = var.github_repo_url
    }
  }

  env_vars = local.oracle_env
}
