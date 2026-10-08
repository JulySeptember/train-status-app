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
