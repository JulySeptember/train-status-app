# Lambda の実行ロールと、付けるポリシーは infra/bootstrap（lambda_role.tf）で作る。
# deploy 用ロールに IAM を変える権限を与えないため（SSM の API キーを読めるロールを引き受けられないように）
data "aws_iam_role" "lambda" {
  name = "${local.name_prefix}-lambda-role"
}

# infra/bootstrap に移したので、state から外すだけにする（AWS のリソースは消さない）
removed {
  from = aws_iam_role.lambda

  lifecycle {
    destroy = false
  }
}

removed {
  from = aws_iam_role_policy_attachment.lambda_basic_execution

  lifecycle {
    destroy = false
  }
}

removed {
  from = aws_iam_role_policy_attachment.lambda_ai

  lifecycle {
    destroy = false
  }
}

removed {
  from = aws_iam_role_policy_attachment.lambda_odpt

  lifecycle {
    destroy = false
  }
}
