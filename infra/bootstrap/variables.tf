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

variable "tags" {
  description = "Common Tags"
  type        = map(string)
  default     = {}
}

variable "github_repository" {
  description = "GitHub Actions からの AssumeRole を許可するリポジトリ（owner/name）"
  type        = string
  default     = "JulySeptember/train-status-app"
}
