variable "aws_region" {
  description = "AWS Region"
  type        = string
}

variable "project_name" {
  description = "Project Name"
  type        = string
}

variable "env" {
  description = "Environment"
  type        = string
}

variable "frontend_bucket_name" {
  description = "Frontend S3 Bucket Name"
  type        = string
}

variable "lambda_artifact_bucket_name" {
  description = "Lambda Artifact Bucket"
  type        = string
}

variable "lambda_artifact_key" {
  description = "Lambda Artifact Key"
  type        = string
}

variable "tags" {
  description = "Common Tags"

  type = map(string)

  default = {}
}

variable "ai_model" {
  description = "AI エージェントが使うモデル。空ならバックエンドの初期値（ai.Models の先頭）"
  type        = string
  default     = ""
}

variable "ai_calls_per_day" {
  description = "アプリ全体の1日の AI 呼び出し回数の上限。Gemini の無料枠（AI Studio で確認する）の 80% にする"
  type        = number
  default     = 200
}

variable "odpt_operators" {
  description = "リアルタイムの情報（運行情報・列車位置）を取る、都営以外の事業者（backend の client.Sources の名前）。空なら都営だけ。チャレンジ限定ライセンスの事業者は 2027-03-12 までに外す"
  type        = list(string)
  default     = []
}
