# ログイン（docs/design/login.md）で、利用者とセッションを読み書きする権限。
# lambda_ai.tf と同じく、Lambda の実行ロールに付ける権限はここで決め、実行ロールへの付与は lambda_role.tf で行う。

locals {
  # infra/main と同じ名前にする
  users_table_name = "${local.name_prefix}-users"
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
}

resource "aws_iam_policy" "lambda_users" {
  name   = "${local.name_prefix}-lambda-users"
  policy = data.aws_iam_policy_document.lambda_users.json

  tags = var.tags
}
