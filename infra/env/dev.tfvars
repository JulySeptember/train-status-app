aws_region = "ap-northeast-1"

project_name = "train-status-app"
env          = "dev"

frontend_bucket_name        = "train-status-app-dev-frontend-assets"
lambda_artifact_bucket_name = "train-status-app-dev-lambda-artifacts"
lambda_artifact_key         = "lambda/bootstrap.zip"

# Gemini 3.5 Flash-Lite の無料枠（AI Studio で確認。2026-10）は RPD 500。その 80%
ai_calls_per_day = 400

# 運行情報・列車位置を取る都営以外の事業者（backend の client.Sources の名前）。小田急・ゆりかもめは運行情報を配信していない。
# チャレンジ限定ライセンスの事業者（JR-East・Keio・Tobu・Keikyu・Tokyu・Seibu）は、許諾の終わる 2027-03-12 までに外す
odpt_operators = [
  "TokyoMetro", "TWR", "MIR", "TamaMonorail",
  "JR-East", "Keio", "Tobu", "Keikyu", "Tokyu", "Seibu",
]

tags = {
  Project     = "train-status-app"
  Environment = "dev"
}