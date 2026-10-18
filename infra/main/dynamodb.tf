# AI エージェントの利用上限（IP ごと・アプリ全体の回数）を数えるテーブル。
# 項目は TTL（expiresAt）で自動で消える
resource "aws_dynamodb_table" "ai_usage" {
  name         = local.ai_usage_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"

  attribute {
    name = "pk"
    type = "S"
  }

  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }

  tags = local.common_tags
}

# ログインした利用者とセッション（docs/design/login.md 6章）。
# セッションは TTL（expiresAt）で自動で消える
resource "aws_dynamodb_table" "users" {
  name         = local.users_table_name
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"
  range_key    = "sk"

  attribute {
    name = "pk"
    type = "S"
  }

  attribute {
    name = "sk"
    type = "S"
  }

  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }

  tags = local.common_tags
}
