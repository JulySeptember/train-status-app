# 運行情報の通知（docs/design/login.md 5.1）。
# API と同じ実行ファイルを APP_MODE=notify で動かし、EventBridge から5分ごとに呼ぶ。
# 終電から始発の間（1時〜5時）は、呼ばれてもすぐに終わる。

resource "aws_cloudwatch_log_group" "notifier" {
  name              = "/aws/lambda/${local.name_prefix}-notifier"
  retention_in_days = 30

  tags = local.common_tags
}

resource "aws_lambda_function" "notifier" {
  function_name = "${local.name_prefix}-notifier"

  role = data.aws_iam_role.lambda.arn

  runtime = "provided.al2023"
  handler = "bootstrap"

  s3_bucket         = var.lambda_artifact_bucket_name
  s3_key            = var.lambda_artifact_key
  s3_object_version = data.aws_s3_object.lambda.version_id

  architectures = ["arm64"]

  # 起動時に API と同じく全事業者のデータを読み込むので、API と同じ大きさにする。
  # 1日 約240回 × 数秒で、無料枠（月 400,000 GB-秒）の 5% ほど
  memory_size = 1024
  timeout     = 60

  publish = false

  environment {
    variables = {
      ENV      = var.env
      APP_MODE = "notify"

      USERS_TABLE         = aws_dynamodb_table.users.name
      VAPID_KEY_PARAMETER = local.vapid_key_parameter

      ODPT_OPERATORS               = join(",", var.odpt_operators)
      ODPT_KEY_PARAMETER           = local.odpt_key_parameter
      ODPT_CHALLENGE_KEY_PARAMETER = local.odpt_challenge_key_parameter
    }
  }

  tags = local.common_tags

  depends_on = [aws_cloudwatch_log_group.notifier]
}

# 失敗しても再実行しない（同じ通知を二重に送らない。次の5分後の実行で確かめ直す）
resource "aws_lambda_function_event_invoke_config" "notifier" {
  function_name                = aws_lambda_function.notifier.function_name
  maximum_retry_attempts       = 0
  maximum_event_age_in_seconds = 300
}

resource "aws_cloudwatch_event_rule" "notifier" {
  name                = "${local.name_prefix}-notifier"
  description         = "運行情報を確かめて、変化があれば Web Push で通知する"
  schedule_expression = "rate(5 minutes)"

  tags = local.common_tags
}

resource "aws_cloudwatch_event_target" "notifier" {
  rule = aws_cloudwatch_event_rule.notifier.name
  arn  = aws_lambda_function.notifier.arn

  retry_policy {
    maximum_retry_attempts       = 0
    maximum_event_age_in_seconds = 300
  }
}

# EventBridge から呼べるようにする（リソースベースのポリシーなので、IAM ロールは要らない）
resource "aws_lambda_permission" "notifier" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.notifier.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.notifier.arn
}
