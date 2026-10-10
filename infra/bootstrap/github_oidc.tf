# GitHub Actions から AWS を操作するためのロール。
# アクセスキーは置かず、OIDC で一時的な認証情報を受け取る。
#   plan:   PR で terraform plan を実行する（読み取りのみ）
#   deploy: main へのマージで backend・infra・frontend を反映する

locals {
  name_prefix = "${var.project_name}-${var.env}"

  github_oidc_host = "token.actions.githubusercontent.com"
}

data "aws_caller_identity" "current" {}

resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://${local.github_oidc_host}"
  client_id_list = ["sts.amazonaws.com"]

  tags = merge(var.tags, {
    Name = "${local.name_prefix}-github-oidc"
  })
}

data "aws_iam_policy_document" "github_assume_role" {
  for_each = {
    # フォークからの PR には ID トークンが発行されないので、同じリポジトリの PR だけが使える
    plan   = "repo:${var.github_repository}:pull_request"
    deploy = "repo:${var.github_repository}:ref:refs/heads/main"
  }

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.github_oidc_host}:aud"
      values   = ["sts.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.github_oidc_host}:sub"
      values   = [each.value]
    }
  }
}

# ============================
# plan
# ============================

resource "aws_iam_role" "github_plan" {
  name               = "${local.name_prefix}-github-plan"
  assume_role_policy = data.aws_iam_policy_document.github_assume_role["plan"].json

  tags = merge(var.tags, {
    Name = "${local.name_prefix}-github-plan"
  })
}

# plan は -lock=false で実行するので、ロック用のテーブルへの書き込みは要らない
resource "aws_iam_role_policy_attachment" "github_plan_readonly" {
  role       = aws_iam_role.github_plan.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

# ============================
# deploy
# ============================

resource "aws_iam_role" "github_deploy" {
  name               = "${local.name_prefix}-github-deploy"
  assume_role_policy = data.aws_iam_policy_document.github_assume_role["deploy"].json

  tags = merge(var.tags, {
    Name = "${local.name_prefix}-github-deploy"
  })
}

# terraform の refresh（既存リソースの読み取り）に使う
resource "aws_iam_role_policy_attachment" "github_deploy_readonly" {
  role       = aws_iam_role.github_deploy.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

# 書き込みは infra/main が管理するリソースだけに絞る
data "aws_iam_policy_document" "github_deploy" {
  statement {
    sid     = "TerraformState"
    actions = ["s3:GetObject", "s3:PutObject"]
    resources = [
      "${aws_s3_bucket.tfstate.arn}/main/terraform.tfstate",
    ]
  }

  statement {
    sid       = "TerraformStateList"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.tfstate.arn]
  }

  statement {
    sid = "TerraformLock"
    actions = [
      "dynamodb:DescribeTable",
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:DeleteItem",
    ]
    resources = [aws_dynamodb_table.terraform_lock.arn]
  }

  # Lambda のアーティファクトと、フロントエンドのバケット
  statement {
    sid     = "S3"
    actions = ["s3:*"]
    resources = [
      aws_s3_bucket.artifact.arn,
      "${aws_s3_bucket.artifact.arn}/*",
      "arn:aws:s3:::${local.name_prefix}-frontend-assets",
      "arn:aws:s3:::${local.name_prefix}-frontend-assets/*",
    ]
  }

  statement {
    sid     = "Lambda"
    actions = ["lambda:*"]
    resources = [
      "arn:aws:lambda:${var.aws_region}:${data.aws_caller_identity.current.account_id}:function:${local.name_prefix}-api",
    ]
  }

  statement {
    sid     = "Logs"
    actions = ["logs:*"]
    resources = [
      "arn:aws:logs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/${local.name_prefix}-*",
    ]
  }

  # Lambda の実行ロールは bootstrap（lambda_role.tf）で作り、ここでは関数に渡すことだけを許す。
  # ロールの作成・信頼ポリシーの書き換え・ポリシーの付け外しを許すと、
  # 自分を信頼させてロールを引き受けたり、強いポリシーを付けたりして権限を広げられる
  statement {
    sid       = "PassLambdaExecutionRole"
    actions   = ["iam:PassRole"]
    resources = [aws_iam_role.lambda.arn]

    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["lambda.amazonaws.com"]
    }
  }

  # AI エージェントの利用上限を数えるテーブル
  statement {
    sid     = "AiUsageTable"
    actions = ["dynamodb:*"]
    resources = [
      "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${local.ai_usage_table_name}",
    ]
  }

  statement {
    sid       = "ApiGateway"
    actions   = ["apigateway:*"]
    resources = ["arn:aws:apigateway:${var.aws_region}::/*"]
  }

  # CloudFront はリソース名で絞れないので、サービス単位で許可する
  statement {
    sid       = "CloudFront"
    actions   = ["cloudfront:*"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "github_deploy" {
  name   = "${local.name_prefix}-github-deploy"
  role   = aws_iam_role.github_deploy.id
  policy = data.aws_iam_policy_document.github_deploy.json
}

# ============================
# plan・deploy 共通
# ============================

# ReadOnlyAccess には ssm:GetParameter（復号つき）が含まれ、SSM の API キー（Gemini・ODPT）まで読めてしまう。
# ワークフローを書き換えてキーを取り出せないよう、明示的に拒否する。
# infra/main はパラメータ名を文字列で Lambda に渡すだけで、aws_ssm_parameter の data source を使っていないので plan・apply には要らない。
data "aws_iam_policy_document" "github_deny_ssm" {
  statement {
    sid     = "DenyReadAppParameters"
    effect  = "Deny"
    actions = ["ssm:GetParameter*"]
    resources = [
      "arn:aws:ssm:*:${data.aws_caller_identity.current.account_id}:parameter/${var.project_name}",
      "arn:aws:ssm:*:${data.aws_caller_identity.current.account_id}:parameter/${var.project_name}/*",
    ]
  }

  # 上の階層（"/" など）を再帰的に読めば、下のパラメータも返ってくるので、パスでの取得はすべて拒否する
  # （/aws/service/... の公開パラメータもパスでは取れなくなる。aws_ssm_parameters_by_path を使うときは見直す）
  statement {
    sid       = "DenyReadParametersByPath"
    effect    = "Deny"
    actions   = ["ssm:GetParametersByPath"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "github_deny_ssm" {
  for_each = {
    plan   = aws_iam_role.github_plan.id
    deploy = aws_iam_role.github_deploy.id
  }

  name   = "${local.name_prefix}-github-deny-ssm"
  role   = each.value
  policy = data.aws_iam_policy_document.github_deny_ssm.json
}
