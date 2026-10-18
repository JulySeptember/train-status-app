# ログイン（docs/design/login.md）で、利用者とセッションを読み書きする権限。
# lambda_ai.tf と同じく、Lambda の実行ロールに付ける権限はここで決め、実行ロールへの付与は lambda_role.tf で行う。

locals {
  # infra/main と同じ名前にする
  users_table_name    = "${local.name_prefix}-users"
  vapid_key_parameter = "/${var.project_name}/${var.env}/vapid-private-key"
}

data "aws_iam_policy_document" "lambda_users" {
  statement {
    sid = "ReadWriteUsers"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:DeleteItem",
      "dynamodb:Query",
      "dynamodb:BatchWriteItem",
      # セッションと目印を TransactWriteItems でまとめて書く（利用者がいることを確かめる）
      "dynamodb:ConditionCheckItem",
    ]
    resources = [
      "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${local.users_table_name}",
    ]
  }

  # 通知: 路線を見ている利用者を GSI から引く
  statement {
    sid     = "QueryUsersIndex"
    actions = ["dynamodb:Query"]
    resources = [
      "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${local.users_table_name}/index/*",
    ]
  }

  # 通知: VAPID の秘密鍵（SecureString）。AWS 管理キー（aws/ssm）で暗号化するので、kms の権限は要らない
  statement {
    sid     = "ReadVapidKey"
    actions = ["ssm:GetParameter"]
    resources = [
      "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${local.vapid_key_parameter}",
    ]
  }
}

resource "aws_iam_policy" "lambda_users" {
  name   = "${local.name_prefix}-lambda-users"
  policy = data.aws_iam_policy_document.lambda_users.json

  tags = var.tags
}
