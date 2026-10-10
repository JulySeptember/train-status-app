# AI エージェント（/api/chat）が使う権限。
# Lambda の実行ロールに付ける権限はここ（手元から適用する bootstrap）で決め、実行ロールへの付与も lambda_role.tf で行う
# （deploy 用ロールに IAM を変える権限を与えず、CD を経由して Lambda の権限を広げられないように）。

locals {
  # infra/main と同じ名前にする
  ai_usage_table_name  = "${local.name_prefix}-ai-usage"
  ai_api_key_parameter = "/${var.project_name}/${var.env}/gemini-api-key"
}

data "aws_iam_policy_document" "lambda_ai" {
  # Gemini の API キー（SecureString）。AWS 管理キー（aws/ssm）で暗号化するので、kms の権限は要らない
  statement {
    sid     = "ReadApiKey"
    actions = ["ssm:GetParameter"]
    resources = [
      "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${local.ai_api_key_parameter}",
    ]
  }

  # 利用上限の回数を数える
  statement {
    sid     = "CountUsage"
    actions = ["dynamodb:UpdateItem"]
    resources = [
      "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${local.ai_usage_table_name}",
    ]
  }
}

resource "aws_iam_policy" "lambda_ai" {
  name   = "${local.name_prefix}-lambda-ai"
  policy = data.aws_iam_policy_document.lambda_ai.json

  tags = var.tags
}
