# 障害と費用の増加に気づくための通知先と、予算のアラート。
# 通知先の SNS トピックは、infra/main の CloudWatch のアラームからも使う（data "aws_sns_topic" で名前から引く）。
# deploy 用ロールに SNS の権限を与えないよう、トピックは bootstrap で作る。
#
# メールの購読は Terraform で作らない（公開リポジトリと tfstate にメールアドレスを残さないため）。
# 作ったあとに手で登録し、届いた確認のメールのリンクを開く（infra/CLAUDE.md）。
# 費用は無料の範囲に収まる: 予算は2つまで、SNS のメール通知は月1,000通まで無料

locals {
  alert_topic_name = "${local.name_prefix}-alerts"
}

resource "aws_sns_topic" "alerts" {
  name = local.alert_topic_name

  tags = merge(var.tags, {
    Name = local.alert_topic_name
  })
}

data "aws_iam_policy_document" "alerts_topic" {
  # アカウントの管理者がトピックを管理・購読できるようにする（既定のポリシーと同じ）
  statement {
    sid = "AccountOwner"
    actions = [
      "sns:GetTopicAttributes",
      "sns:SetTopicAttributes",
      "sns:AddPermission",
      "sns:RemovePermission",
      "sns:DeleteTopic",
      "sns:Subscribe",
      "sns:ListSubscriptionsByTopic",
      "sns:Publish",
    ]
    resources = [aws_sns_topic.alerts.arn]

    principals {
      type        = "AWS"
      identifiers = ["*"]
    }

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceOwner"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }

  # 予算（AWS Budgets）と CloudWatch のアラームが、このアカウントの分だけ通知できるようにする
  statement {
    sid       = "AlertServices"
    actions   = ["sns:Publish"]
    resources = [aws_sns_topic.alerts.arn]

    principals {
      type        = "Service"
      identifiers = ["budgets.amazonaws.com", "cloudwatch.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
}

resource "aws_sns_topic_policy" "alerts" {
  arn    = aws_sns_topic.alerts.arn
  policy = data.aws_iam_policy_document.alerts_topic.json
}

# アカウント全体の月の費用。無料で動かすのが前提なので、少しでも費用が出たら気づけるようにする
resource "aws_budgets_budget" "monthly" {
  name         = "${local.name_prefix}-monthly"
  budget_type  = "COST"
  limit_amount = "5"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  # 実績が $1 を超えた
  notification {
    comparison_operator       = "GREATER_THAN"
    threshold                 = 20
    threshold_type            = "PERCENTAGE"
    notification_type         = "ACTUAL"
    subscriber_sns_topic_arns = [aws_sns_topic.alerts.arn]
  }

  # 実績が $5 を超えた
  notification {
    comparison_operator       = "GREATER_THAN"
    threshold                 = 100
    threshold_type            = "PERCENTAGE"
    notification_type         = "ACTUAL"
    subscriber_sns_topic_arns = [aws_sns_topic.alerts.arn]
  }

  # 月末までに $5 を超えそう
  notification {
    comparison_operator       = "GREATER_THAN"
    threshold                 = 100
    threshold_type            = "PERCENTAGE"
    notification_type         = "FORECASTED"
    subscriber_sns_topic_arns = [aws_sns_topic.alerts.arn]
  }

  depends_on = [aws_sns_topic_policy.alerts]

  tags = var.tags
}
