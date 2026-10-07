#!/bin/sh

set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
ASSETS_DIR="${ROOT}/assets"

mkdir -p "${ASSETS_DIR}"

BASE_URL="https://api-public.odpt.org/api/v4"

download() {
    endpoint="$1"
    filename="$2"

    echo "Downloading ${filename}..."

    curl \
        --fail \
        --silent \
        --show-error \
        "${BASE_URL}/${endpoint}?odpt:operator=odpt.Operator:Toei" \
        -o "${ASSETS_DIR}/${filename}"
}

download "odpt:Railway"          "railway.json"
download "odpt:Station"          "station.json"
download "odpt:RailwayFare"      "railway_fare.json"
download "odpt:PassengerSurvey"  "passenger_survey.json"
download "odpt:TrainType"        "train_type.json"
download "odpt:StationTimetable" "station_timetable.json"

# 列車時刻表は 1,000 件を超えて途中で切れるため、全件ダウンロード用の URL から取得する
# （公開 API の全件版は都営のデータだけを含む）。保存先へのリダイレクトを返すので --location を付ける
echo "Downloading train_timetable.json..."

curl \
    --fail \
    --location \
    --silent \
    --show-error \
    "${BASE_URL}/odpt:TrainTimetable.json" \
    -o "${ASSETS_DIR}/train_timetable.json"

echo
echo "Assets updated successfully. Run 'go generate ./assets' to regenerate the gob files."

ls -lh "${ASSETS_DIR}"