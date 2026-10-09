aws_region = "ap-northeast-1"

project_name = "train-status-app"
env          = "dev"

frontend_bucket_name        = "train-status-app-dev-frontend-assets"
lambda_artifact_bucket_name = "train-status-app-dev-lambda-artifacts"
lambda_artifact_key         = "lambda/bootstrap.zip"

# Gemini 3.5 Flash-Lite の無料枠（AI Studio で確認。2026-10）は RPD 500。その 80%
ai_calls_per_day = 400

# 運行情報・列車位置を取る都営以外の事業者（backend の client.Sources の名前）。小田急・ゆりかもめは運行情報を配信していない。
# チャレンジ限定ライセンスの事業者（JR-East・Keio・Tobu・Keikyu・Tokyu・Seibu）は、許諾の終わる 2027-03-12 までに外す。
# ここから外しても、埋め込んだ駅・時刻表は配信され続ける。あわせて assets/extra からその事業者（小田急を含む）のデータを除いた版を作り
# （make backend-extra-upload）、extra.version を差し替え、S3 の古い版も消す（docs/design/multi-operator.md 10.2）
odpt_operators = [
  "TokyoMetro", "TWR", "MIR", "TamaMonorail",
  "JR-East", "Keio", "Tobu", "Keikyu", "Tokyu", "Seibu",
]

tags = {
  Project     = "train-status-app"
  Environment = "dev"
}