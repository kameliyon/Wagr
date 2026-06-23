# ── Render ─────────────────────────────────────────────────────────────────────

variable "render_api_key" {
  description = "Render API key (Account > API Keys)"
  type        = string
  sensitive   = true
}

variable "render_owner_id" {
  description = "Render workspace owner ID — 'usr-...' for personal, 'tea-...' for a team (Account Settings URL)"
  type        = string
}

# ── Cloudflare ─────────────────────────────────────────────────────────────────

variable "cloudflare_api_token" {
  description = "Cloudflare API token scoped to 'Cloudflare Pages: Edit' and 'DNS: Edit'"
  type        = string
  sensitive   = true
}

variable "cloudflare_account_id" {
  description = "Cloudflare account ID (visible in the dashboard URL and Overview page)"
  type        = string
}

variable "cloudflare_zone_id" {
  description = "Cloudflare zone ID for wagrs.app (DNS > Zone ID in the dashboard)"
  type        = string
}

# ── General ────────────────────────────────────────────────────────────────────

variable "environment" {
  description = "Deployment environment label used in resource names"
  type        = string
  default     = "production"
}

variable "render_region" {
  description = "Render region for all services (oregon, frankfurt, ohio, singapore, virginia)"
  type        = string
  default     = "oregon"
}

variable "github_repo_url" {
  description = "Full GitHub repo URL, e.g. https://github.com/your-org/Wagr"
  type        = string
}

variable "github_owner" {
  description = "GitHub organization or username that owns the repo"
  type        = string
}

variable "github_repo_name" {
  description = "GitHub repository name (without owner), e.g. Wagr"
  type        = string
  default     = "Wagr"
}

# ── Database ───────────────────────────────────────────────────────────────────

variable "database_url" {
  description = "Internal Database URL from Render PostgreSQL dashboard"
  type        = string
  sensitive   = true
}

# ── App secrets ────────────────────────────────────────────────────────────────

variable "jwt_secret" {
  description = "JWT signing secret (min 32 random characters)"
  type        = string
  sensitive   = true
}

variable "hedera_network" {
  description = "Hedera network: testnet or mainnet"
  type        = string
  default     = "testnet"
}

variable "hedera_usdc_token_id" {
  description = "Hedera USDC token ID (e.g. 0.0.429274 on testnet)"
  type        = string
}

variable "hedera_escrow_contract_id" {
  description = "Deployed LeagueEscrow contract ID on Hedera (e.g. 0.0.9177767)"
  type        = string
}

variable "hedera_operator_id" {
  description = "Hedera operator account ID used for payout execution (e.g. 0.0.12345)"
  type        = string
  sensitive   = true
}

variable "hedera_operator_key" {
  description = "Hedera operator private key in DER-encoded hex format"
  type        = string
  sensitive   = true
}

variable "oracle_job_timeout" {
  description = "Maximum duration for oracle cron job runs (Go duration string, e.g. 15m)"
  type        = string
  default     = "15m"
}

# ── Frontend ───────────────────────────────────────────────────────────────────

variable "vite_walletconnect_project_id" {
  description = "WalletConnect Project ID from cloud.walletconnect.com"
  type        = string
  sensitive   = true
}
