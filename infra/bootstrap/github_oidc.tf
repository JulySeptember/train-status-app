# GitHub Actions から AWS を操作するためのロール。
# アクセスキーは置かず、OIDC で一時的な認証情報を受け取る。
#   plan:   PR で terraform plan を実行する（読み取りのみ）
#   deploy: main へのマージで backend・infra・frontend を反映する

locals {
  name_prefix = "${var.project_name}-${var.env}"

  github_oidc_host = "token.actions.githubusercontent.com"

  # infra/env/dev.tfvars の lambda_artifact_key・Makefile の LAMBDA_ARTIFACT_KEY と合わせる
  lambda_artifact_key = "lambda/bootstrap.zip"
  # 都営以外の事業者のデータの置き場所（Makefile の EXTRA_PREFIX）
  extra_assets_prefix = "assets-extra"

  app_table_arns = [
    "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${local.ai_usage_table_name}",
    "arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${local.users_table_name}",
  ]
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

  # Lambda のアーティファクト用バケットは bootstrap の管理なので、backend-upload で zip を置くことだけを許す
  # （バケットポリシー・公開ブロックの変更や削除はさせない）。読み取りは ReadOnlyAccess で足りる
  statement {
    sid = "LambdaArtifactUpload"
    actions = [
      "s3:PutObject",
      "s3:AbortMultipartUpload",
    ]
    resources = ["${aws_s3_bucket.artifact.arn}/${local.lambda_artifact_key}"]
  }

  # フロントエンドのバケットは infra/main が作って管理する
  statement {
    sid     = "FrontendBucket"
    actions = ["s3:*"]
    resources = [
      "arn:aws:s3:::${local.name_prefix}-frontend-assets",
      "arn:aws:s3:::${local.name_prefix}-frontend-assets/*",
    ]
  }

  statement {
    sid     = "Lambda"
    actions = ["lambda:*"]
    resources = [
      "arn:aws:lambda:${var.aws_region}:${data.aws_caller_identity.current.account_id}:function:${local.name_prefix}-api",
      "arn:aws:lambda:${var.aws_region}:${data.aws_caller_identity.current.account_id}:function:${local.name_prefix}-notifier",
    ]
  }

  # 運行情報の通知を5分ごとに呼ぶ EventBridge のルール（ターゲットは Lambda なので、ロールを渡さない）
  statement {
    sid     = "EventBridgeRules"
    actions = ["events:*"]
    resources = [
      "arn:aws:events:${var.aws_region}:${data.aws_caller_identity.current.account_id}:rule/${local.name_prefix}-*",
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

  # AI エージェントの利用上限を数えるテーブルと、ログインした利用者のテーブル。
  # Terraform がテーブルを作って設定を変える操作だけを許し、項目の読み書き・バックアップ・エクスポートはさせない
  statement {
    sid = "AppTables"
    actions = [
      "dynamodb:CreateTable",
      "dynamodb:UpdateTable",
      "dynamodb:DeleteTable",
      "dynamodb:UpdateTimeToLive",
      "dynamodb:TagResource",
      "dynamodb:UntagResource",
    ]
    resources = local.app_table_arns
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

# ReadOnlyAccess には dynamodb:Scan などが含まれ、ログインした利用者のデータ（Google のユーザー ID・お気に入り・
# Push の購読先）と AI の利用回数まで読めてしまう。plan・apply にはテーブルの設定（Describe*）だけがあればよい
data "aws_iam_policy_document" "github_deny_app_data" {
  statement {
    sid    = "DenyReadAppTableItems"
    effect = "Deny"
    actions = [
      "dynamodb:GetItem",
      "dynamodb:BatchGetItem",
      "dynamodb:Query",
      "dynamodb:Scan",
      "dynamodb:PartiQLSelect",
    ]
    resources = concat(local.app_table_arns, [for arn in local.app_table_arns : "${arn}/index/*"])
  }

  # deploy は UpdateTable で Streams を有効にできるので、ストリームから書き込まれた項目を読むことも拒否する
  statement {
    sid    = "DenyReadAppTableStreams"
    effect = "Deny"
    actions = [
      "dynamodb:GetShardIterator",
      "dynamodb:GetRecords",
    ]
    resources = [for arn in local.app_table_arns : "${arn}/stream/*"]
  }

  # users テーブルは deletion_protection_enabled で守っているが、UpdateTable で外せるので、削除そのものも拒否する
  statement {
    sid       = "DenyDeleteUsersTable"
    effect    = "Deny"
    actions   = ["dynamodb:DeleteTable"]
    resources = ["arn:aws:dynamodb:${var.aws_region}:${data.aws_caller_identity.current.account_id}:table/${local.users_table_name}"]
  }
}

resource "aws_iam_role_policy" "github_deny_app_data" {
  for_each = {
    plan   = aws_iam_role.github_plan.id
    deploy = aws_iam_role.github_deploy.id
  }

  name   = "${local.name_prefix}-github-deny-app-data"
  role   = each.value
  policy = data.aws_iam_policy_document.github_deny_app_data.json
}

# 都営以外の事業者のデータ（ODPT のライセンスの第8条4項で第三者への公開が禁じられている）は、
# deploy の backend-extra-download だけが読む。plan に要るのは tfstate と lambda/bootstrap.zip なので、plan では拒否する。
# ただし bootstrap.zip のバイナリ（と lambda:GetFunction が返すコードの URL）には他社データが埋め込まれていて、
# plan の data "aws_s3_object"（HeadObject は s3:GetObject の権限）と Lambda の refresh に要るため、そちらは塞げない。
# plan ロールを引き受けられるのがこのリポジトリに push できる人だけ（フォークの PR には ID トークンが出ない）であることが前提
data "aws_iam_policy_document" "github_plan_deny_extra_assets" {
  statement {
    sid       = "DenyReadExtraAssets"
    effect    = "Deny"
    actions   = ["s3:GetObject*"]
    resources = ["${aws_s3_bucket.artifact.arn}/${local.extra_assets_prefix}/*"]
  }
}

resource "aws_iam_role_policy" "github_plan_deny_extra_assets" {
  name   = "${local.name_prefix}-github-plan-deny-extra-assets"
  role   = aws_iam_role.github_plan.id
  policy = data.aws_iam_policy_document.github_plan_deny_extra_assets.json
}
