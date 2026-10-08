data "aws_s3_object" "lambda" {
  bucket = var.lambda_artifact_bucket_name
  key    = var.lambda_artifact_key
}

# Lambda が自動で作ったロググループを取り込んで、保持期間を設定する
import {
  to = aws_cloudwatch_log_group.lambda
  id = "/aws/lambda/${local.name_prefix}-api"
}

resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${local.name_prefix}-api"
  retention_in_days = 30

  tags = local.common_tags
}

resource "aws_lambda_function" "this" {
  function_name = "${local.name_prefix}-api"

  role = aws_iam_role.lambda.arn

  runtime = "provided.al2023"
  handler = "bootstrap"

  s3_bucket         = var.lambda_artifact_bucket_name
  s3_key            = var.lambda_artifact_key
  s3_object_version = data.aws_s3_object.lambda.version_id

  architectures = ["arm64"]

  memory_size = 256
  timeout     = 30

  publish = false

  environment {
    variables = {
      ENV = var.env

      # AI エージェント。キーの値ではなく、キーを入れた SSM のパラメータ名を渡す
      AI_API_KEY_PARAMETER   = local.ai_api_key_parameter
      AI_USAGE_TABLE         = aws_dynamodb_table.ai_usage.name
      AI_MODEL               = var.ai_model
      AI_LIMIT_CALLS_PER_DAY = tostring(var.ai_calls_per_day)
    }
  }

  tags = local.common_tags

  depends_on = [aws_cloudwatch_log_group.lambda]
}
