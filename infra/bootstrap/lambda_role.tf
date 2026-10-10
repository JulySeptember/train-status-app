# Lambda の実行ロール。
# 以前は infra/main で作っていたが、deploy 用ロールが信頼ポリシーを書き換えられると、
# 自分を信頼させてこのロールを引き受け、SSM の API キーを読めてしまう。
# ロールと付けるポリシーはここ（手元から適用する bootstrap）で決め、deploy 用ロールには iam:PassRole だけを許す。

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

  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "lambda" {
  for_each = {
    basic_execution = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
    ai              = aws_iam_policy.lambda_ai.arn
    odpt            = aws_iam_policy.lambda_odpt.arn
  }

  role       = aws_iam_role.lambda.name
  policy_arn = each.value
}

# infra/main で作ったものを取り込む。
# 実行ロールが無い新しい環境（destroy の後など）では import が失敗するので、この import ブロックを消してから適用する
import {
  to = aws_iam_role.lambda
  id = "${local.name_prefix}-lambda-role"
}

import {
  to = aws_iam_role_policy_attachment.lambda["basic_execution"]
  id = "${local.name_prefix}-lambda-role/arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

import {
  to = aws_iam_role_policy_attachment.lambda["ai"]
  id = "${local.name_prefix}-lambda-role/arn:aws:iam::${data.aws_caller_identity.current.account_id}:policy/${local.name_prefix}-lambda-ai"
}

import {
  to = aws_iam_role_policy_attachment.lambda["odpt"]
  id = "${local.name_prefix}-lambda-role/arn:aws:iam::${data.aws_caller_identity.current.account_id}:policy/${local.name_prefix}-lambda-odpt"
}
