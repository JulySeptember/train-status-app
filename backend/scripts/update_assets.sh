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
download "odpt:StationTimetable" "station_timetable.json"
download "odpt:TrainTimetable"   "train_timetable.json"

echo
echo "Assets updated successfully."

ls -lh "${ASSETS_DIR}"