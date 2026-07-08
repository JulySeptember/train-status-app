data "aws_s3_object" "lambda" {
  bucket = var.lambda_artifact_bucket_name
  key    = var.lambda_artifact_key
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
    }
  }

  tags = local.common_tags
}
