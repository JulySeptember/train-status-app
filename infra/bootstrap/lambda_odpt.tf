# 都営以外の事業者のリアルタイムの情報を取るのに使う ODPT のキーを読む権限。
# キーは SSM Parameter Store（SecureString）に手で登録する（docs/design/multi-operator.md 4.3）。
# lambda_ai.tf と同じく、Lambda の実行ロールに付ける権限はここで決め、実行ロールへの付与は lambda_role.tf で行う。

locals {
  # infra/main と同じ名前にする
  odpt_key_parameter           = "/${var.project_name}/${var.env}/odpt-consumer-key"
  odpt_challenge_key_parameter = "/${var.project_name}/${var.env}/odpt-challenge-consumer-key"
}

data "aws_iam_policy_document" "lambda_odpt" {
  # AWS 管理キー（aws/ssm）で暗号化するので、kms の権限は要らない
  statement {
    sid     = "ReadOdptKeys"
    actions = ["ssm:GetParameter"]
    resources = [
      "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${local.odpt_key_parameter}",
      "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${local.odpt_challenge_key_parameter}",
    ]
  }
}

resource "aws_iam_policy" "lambda_odpt" {
  name   = "${local.name_prefix}-lambda-odpt"
  policy = data.aws_iam_policy_document.lambda_odpt.json

  tags = var.tags
}
