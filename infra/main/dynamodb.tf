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

  # 利用者のデータを持つので、誤って消したり作り直したりしないようにする
  deletion_protection_enabled = true

  attribute {
    name = "pk"
    type = "S"
  }

  attribute {
    name = "sk"
    type = "S"
  }

  # 通知の条件（USER#<sub> / ALERT#<路線ID>）だけが持つ。路線を見ている利用者を引く
  attribute {
    name = "gsi1pk"
    type = "S"
  }

  attribute {
    name = "gsi1sk"
    type = "S"
  }

  global_secondary_index {
    name            = "gsi1"
    projection_type = "ALL"

    key_schema {
      attribute_name = "gsi1pk"
      key_type       = "HASH"
    }

    key_schema {
      attribute_name = "gsi1sk"
      key_type       = "RANGE"
    }
  }

  ttl {
    attribute_name = "expiresAt"
    enabled        = true
  }

  tags = local.common_tags
}
