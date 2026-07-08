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
