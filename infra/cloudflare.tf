# ── Cloudflare Pages — frontend ────────────────────────────────────────────────

resource "cloudflare_pages_project" "wagr_frontend" {
  account_id        = var.cloudflare_account_id
  name              = "wagr-${var.environment}"
  production_branch = "master"

  build_config = {
    build_command   = "npm run build"
    destination_dir = "dist"
    root_dir        = "src/web" # Vite project lives here, not the repo root
  }

  source = {
    type = "github"
    config = {
      owner                         = var.github_owner
      repo_name                     = var.github_repo_name
      production_branch             = "master"
      pr_comments_enabled           = true
      production_deployment_enabled = true
    }
  }

  deployment_configs = {
    production = {
      environment_variables = {
        # VITE_* variables are baked into the JS bundle at build time.
        # Changing these requires triggering a new build.
        VITE_API_URL                   = "https://api.wagrs.app"
        VITE_HEDERA_ESCROW_CONTRACT_ID = var.hedera_escrow_contract_id
        VITE_HEDERA_USDC_TOKEN_ID      = var.hedera_usdc_token_id
        VITE_WALLETCONNECT_PROJECT_ID  = var.vite_walletconnect_project_id
      }
    }
    preview = {
      environment_variables = {
        # PR preview deployments hit the same Render backend.
        VITE_API_URL                   = render_web_service.gateway.url
        VITE_HEDERA_ESCROW_CONTRACT_ID = var.hedera_escrow_contract_id
        VITE_HEDERA_USDC_TOKEN_ID      = var.hedera_usdc_token_id
        VITE_WALLETCONNECT_PROJECT_ID  = var.vite_walletconnect_project_id
      }
    }
  }
}

# ── Custom domain — apex ────────────────────────────────────────────────────────
# Cloudflare Pages handles its own CNAME/ALIAS for Pages domains automatically
# once the domain is attached. You do NOT need a separate dns_record for these.

resource "cloudflare_pages_domain" "apex" {
  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.wagr_frontend.name
  name         = "wagrs.app"
}

resource "cloudflare_pages_domain" "www" {
  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.wagr_frontend.name
  name         = "www.wagrs.app"
}

# ── DNS — api subdomain → Render ───────────────────────────────────────────────
# DNS-only (proxied = false) keeps API latency low and avoids Cloudflare's
# 100-second proxy timeout limit on long-running requests.

resource "cloudflare_dns_record" "api" {
  zone_id = var.cloudflare_zone_id
  name    = "api"
  type    = "CNAME"
  content = trimprefix(render_web_service.gateway.url, "https://")
  ttl     = 1       # 1 = automatic TTL (Cloudflare-managed)
  proxied = false
}
