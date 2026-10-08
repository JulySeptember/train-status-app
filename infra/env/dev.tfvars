aws_region = "ap-northeast-1"

project_name = "train-status-app"
env          = "dev"

frontend_bucket_name        = "train-status-app-dev-frontend-assets"
lambda_artifact_bucket_name = "train-status-app-dev-lambda-artifacts"
lambda_artifact_key         = "lambda/bootstrap.zip"

# Gemini 3.5 Flash-Lite の無料枠（AI Studio で確認。2026-10）は RPD 500。その 80%
ai_calls_per_day = 400

tags = {
  Project     = "train-status-app"
  Environment = "dev"
}