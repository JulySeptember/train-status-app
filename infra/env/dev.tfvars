aws_region = "ap-northeast-1"

project_name = "train-status-app"
env          = "dev"

frontend_bucket_name        = "train-status-app-dev-frontend-assets"
lambda_artifact_bucket_name = "train-status-app-dev-lambda-artifacts"
lambda_artifact_key         = "lambda/bootstrap.zip"

tags = {
  Project     = "train-status-app"
  Environment = "dev"
}