locals {

  name_prefix = "${var.project_name}-${var.env}"

  # infra/bootstrap（Lambda の AI 用のポリシー）と同じ名前にする。
  # API キーのパラメータは Terraform で作らず、手で登録する（値を tfstate に残さないため）
  ai_usage_table_name  = "${local.name_prefix}-ai-usage"
  ai_api_key_parameter = "/${var.project_name}/${var.env}/gemini-api-key"

  common_tags = merge(
    var.tags,
    {
      Project     = var.project_name
      Environment = var.env
    }
  )

}
