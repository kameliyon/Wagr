resource "random_password" "db" {
  length  = 32
  special = false
}

# DB password — auto-generated on first apply; rotate manually via AWS console/CLI
resource "aws_secretsmanager_secret" "db_password" {
  name = "wagr/${var.environment}/db-password"
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id     = aws_secretsmanager_secret.db_password.id
  secret_string = random_password.db.result

  lifecycle {
    ignore_changes = [secret_string]
  }
}

# Hedera operator credentials — set after apply via AWS console or CLI:
#   aws secretsmanager put-secret-value --secret-id wagr/dev/hedera-operator-id --secret-string "0.0.XXXXX"
#   aws secretsmanager put-secret-value --secret-id wagr/dev/hedera-operator-key --secret-string "302e..."
resource "aws_secretsmanager_secret" "hedera_operator_id" {
  name = "wagr/${var.environment}/hedera-operator-id"
}

resource "aws_secretsmanager_secret_version" "hedera_operator_id" {
  secret_id     = aws_secretsmanager_secret.hedera_operator_id.id
  secret_string = "REPLACE_ME"

  lifecycle {
    ignore_changes = [secret_string]
  }
}

resource "aws_secretsmanager_secret" "hedera_operator_key" {
  name = "wagr/${var.environment}/hedera-operator-key"
}

resource "aws_secretsmanager_secret_version" "hedera_operator_key" {
  secret_id     = aws_secretsmanager_secret.hedera_operator_key.id
  secret_string = "REPLACE_ME"

  lifecycle {
    ignore_changes = [secret_string]
  }
}

# JWT signing secret — set after apply:
#   aws secretsmanager put-secret-value --secret-id wagr/dev/jwt-secret --secret-string "$(openssl rand -hex 32)"
resource "aws_secretsmanager_secret" "jwt_secret" {
  name = "wagr/${var.environment}/jwt-secret"
}

resource "aws_secretsmanager_secret_version" "jwt_secret" {
  secret_id     = aws_secretsmanager_secret.jwt_secret.id
  secret_string = "REPLACE_ME"

  lifecycle {
    ignore_changes = [secret_string]
  }
}
