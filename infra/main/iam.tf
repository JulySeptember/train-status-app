# Lambda の実行ロールと、付けるポリシーは infra/bootstrap（lambda_role.tf）で作る。
# deploy 用ロールに IAM を変える権限を与えないため（SSM の API キーを読めるロールを引き受けられないように）
data "aws_iam_role" "lambda" {
  name = "${local.name_prefix}-lambda-role"
}
