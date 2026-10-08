data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${local.name_prefix}-lambda-role"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json

  tags = local.common_tags
}

resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# AI エージェントの権限（API キーの読み取り・利用上限の書き込み）。ポリシーは infra/bootstrap で作る
data "aws_iam_policy" "lambda_ai" {
  name = "${local.name_prefix}-lambda-ai"
}

resource "aws_iam_role_policy_attachment" "lambda_ai" {
  role       = aws_iam_role.lambda.name
  policy_arn = data.aws_iam_policy.lambda_ai.arn
}
