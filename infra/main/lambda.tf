data "aws_s3_object" "lambda" {
  bucket = var.lambda_artifact_bucket_name
  key    = var.lambda_artifact_key
}

resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${local.name_prefix}-api"
  retention_in_days = 30

  tags = local.common_tags
}

resource "aws_lambda_function" "this" {
  function_name = "${local.name_prefix}-api"

  role = data.aws_iam_role.lambda.arn

  runtime = "provided.al2023"
  handler = "bootstrap"

  s3_bucket         = var.lambda_artifact_bucket_name
  s3_key            = var.lambda_artifact_key
  s3_object_version = data.aws_s3_object.lambda.version_id

  architectures = ["arm64"]

  # 都営以外の事業者のデータで約260MB を使う（docs/design/multi-operator.md 11.1）。
  # CPU はメモリに比例して割り当てられるので、起動（直通運転の組を作る）と最初の検索（路線網を作る）を
  # 速くするために 1024MB にする（512MB では Init Duration 約1.1秒・最初の検索 約1.1秒だった）。
  # 直近30日の実行時間（約480秒）なら、無料枠（月 400,000 GB-秒）の 1% 未満
  memory_size = 1024
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

      # 都営以外の事業者。キーの値ではなく、キーを入れた SSM のパラメータ名を渡す
      ODPT_OPERATORS               = join(",", var.odpt_operators)
      ODPT_KEY_PARAMETER           = local.odpt_key_parameter
      ODPT_CHALLENGE_KEY_PARAMETER = local.odpt_challenge_key_parameter
    }
  }

  tags = local.common_tags

  depends_on = [aws_cloudwatch_log_group.lambda]
}
