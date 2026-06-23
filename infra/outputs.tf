output "gateway_render_url" {
  description = "Render web service URL (internal *.onrender.com URL)"
  value       = render_web_service.gateway.url
}

output "gateway_url" {
  description = "Public API gateway URL via wagrs.app custom domain"
  value       = "https://api.wagrs.app"
}

output "frontend_url" {
  description = "Public frontend URL"
  value       = "https://wagrs.app"
}

output "pages_default_url" {
  description = "Cloudflare Pages default subdomain URL (always works even before DNS cutover)"
  value       = "https://${cloudflare_pages_project.wagr_frontend.name}.pages.dev"
}

