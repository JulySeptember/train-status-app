# 障害に気づくための CloudWatch のアラームと、API Gateway のアクセスログ。
# 通知先の SNS トピックと予算のアラートは infra/bootstrap（monitoring.tf）で作る。
# 費用は無料の範囲に収まる: 標準のアラームは10個まで、Logs は取り込み・保存とも月 5GB まで無料。
# アラームを足すときは10個を超えないようにする（異常検出のアラームは1つで3個分に数えられる）

data "aws_sns_topic" "alerts" {
  name = "${local.name_prefix}-alerts"
}

locals {
  alarm_actions = [data.aws_sns_topic.alerts.arn]
}

# ============================
# API Gateway のアクセスログ
# ============================

# API Gateway は CloudFront を通さずにも呼べるので、誰がどれだけ呼んだか、429 がどのリクエストかを追えるようにする。
# CloudFront を通ったものは userAgent が "Amazon CloudFront" になる。
# path にクエリは入らない（AI への入力は POST の本文なので、ここには出ない）
resource "aws_cloudwatch_log_group" "api_access" {
  name              = "/aws/apigateway/${local.name_prefix}-api"
  retention_in_days = 30

  tags = local.common_tags
}

locals {
  api_access_log_format = jsonencode({
    requestId          = "$context.requestId"
    requestTime        = "$context.requestTime"
    sourceIp           = "$context.identity.sourceIp"
    userAgent          = "$context.identity.userAgent"
    method             = "$context.httpMethod"
    path               = "$context.path"
    status             = "$context.status"
    responseLength     = "$context.responseLength"
    responseLatency    = "$context.responseLatency"
    integrationStatus  = "$context.integrationStatus"
    integrationLatency = "$context.integrationLatency"
    errorMessage       = "$context.error.message"
  })
}

# ============================
# アラーム
# ============================

# API の Lambda が異常終了した（panic は Recovery が 500 にするので、タイムアウトやメモリ不足など）
resource "aws_cloudwatch_metric_alarm" "api_errors" {
  alarm_name        = "${local.name_prefix}-api-errors"
  alarm_description = "API の Lambda がエラーで終わった。/aws/lambda/${local.name_prefix}-api のログを見る"

  namespace   = "AWS/Lambda"
  metric_name = "Errors"
  dimensions  = { FunctionName = aws_lambda_function.this.function_name }
  statistic   = "Sum"
  period      = 300

  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 1
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"

  alarm_actions = local.alarm_actions
  ok_actions    = local.alarm_actions

  tags = local.common_tags
}

# 通知の Lambda が失敗し続けている。再試行しない設定（notifier.tf）なので、1回の失敗は次の5分後に確かめ直される。
# ODPT の一時的な失敗では鳴らさず、3回（15分）続けて失敗したら知らせる
resource "aws_cloudwatch_metric_alarm" "notifier_errors" {
  alarm_name        = "${local.name_prefix}-notifier-errors"
  alarm_description = "運行情報の通知の Lambda が15分続けて失敗している。/aws/lambda/${local.name_prefix}-notifier のログを見る"

  namespace   = "AWS/Lambda"
  metric_name = "Errors"
  dimensions  = { FunctionName = aws_lambda_function.notifier.function_name }
  statistic   = "Sum"
  period      = 300

  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 1
  evaluation_periods  = 3
  datapoints_to_alarm = 3
  treat_missing_data  = "notBreaching"

  alarm_actions = local.alarm_actions
  ok_actions    = local.alarm_actions

  tags = local.common_tags
}

# API の Lambda が同時実行数の上限で断られた
resource "aws_cloudwatch_metric_alarm" "api_throttles" {
  alarm_name        = "${local.name_prefix}-api-throttles"
  alarm_description = "API の Lambda がスロットリングされた（同時実行数の上限）。アクセスログで呼び出し元を見る"

  namespace   = "AWS/Lambda"
  metric_name = "Throttles"
  dimensions  = { FunctionName = aws_lambda_function.this.function_name }
  statistic   = "Sum"
  period      = 300

  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 1
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"

  alarm_actions = local.alarm_actions
  ok_actions    = local.alarm_actions

  tags = local.common_tags
}

# API の Lambda の実行時間の p99 が、タイムアウト（30秒）の 1/3 を10分続けて超えた。
# 経路検索・AI でも普段は数秒で終わる
resource "aws_cloudwatch_metric_alarm" "api_duration_p99" {
  alarm_name        = "${local.name_prefix}-api-duration-p99"
  alarm_description = "API の Lambda の実行時間の p99 が10秒を超えている。ODPT・Gemini の応答の遅れか、処理の重さを疑う"

  namespace          = "AWS/Lambda"
  metric_name        = "Duration"
  dimensions         = { FunctionName = aws_lambda_function.this.function_name }
  extended_statistic = "p99"
  period             = 300

  comparison_operator = "GreaterThanThreshold"
  threshold           = 10000
  evaluation_periods  = 2
  datapoints_to_alarm = 2
  treat_missing_data  = "notBreaching"

  alarm_actions = local.alarm_actions
  ok_actions    = local.alarm_actions

  tags = local.common_tags
}

# API Gateway が 5xx を返した（Lambda の 500・502 と、API Gateway 自身の失敗）。
# ODPT が落ちたときの 502 でも鳴る。散発的なものでは鳴らさないよう、5分で5回以上にする
resource "aws_cloudwatch_metric_alarm" "api_5xx" {
  alarm_name        = "${local.name_prefix}-api-5xx"
  alarm_description = "API Gateway が5分で5回以上 5xx を返した。アクセスログと Lambda のログでステータスを見る"

  namespace   = "AWS/ApiGateway"
  metric_name = "5xx"
  dimensions  = { ApiId = aws_apigatewayv2_api.this.id }
  statistic   = "Sum"
  period      = 300

  comparison_operator = "GreaterThanOrEqualToThreshold"
  threshold           = 5
  evaluation_periods  = 1
  treat_missing_data  = "notBreaching"

  alarm_actions = local.alarm_actions
  ok_actions    = local.alarm_actions

  tags = local.common_tags
}
