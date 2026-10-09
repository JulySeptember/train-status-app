#!/bin/sh

set -eu

# 使い方:
#   scripts/update_assets.sh               都営と、都営以外の事業者のデータを取り直す
#   scripts/update_assets.sh --extra-only  都営以外の事業者のデータだけを取り直す
#
# 都営以外の事業者のデータは、backend/.env の ODPT_CONSUMER_KEY・ODPT_CHALLENGE_CONSUMER_KEY を使って取り、
# 都内に絞って assets/extra に置く（コミットしない。assets/extra/README.md）。
# キーは URL のクエリに入るので、URL を画面に出さない。

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
ASSETS_DIR="${ROOT}/assets"

mkdir -p "${ASSETS_DIR}"

EXTRA_ONLY=false
if [ "${1:-}" = "--extra-only" ]; then
    EXTRA_ONLY=true
fi

# =========================
# 都営（キー不要。CC BY 4.0 なので assets に置いてコミットする）
# =========================

update_toei() {
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
    echo "Toei assets updated. Run 'go generate ./assets' to regenerate the gob files."
}

# =========================
# 都営以外の事業者（キーが要る。assets/extra に置き、コミットしない）
# =========================

# 都内に駅を持つ事業者（docs/design/multi-operator.md 2.2）
BASIC_OPERATORS="TokyoMetro TWR MIR TamaMonorail Yurikamome"
CHALLENGE_OPERATORS="JR-East Keio Tobu Keikyu Tokyu Seibu Odakyu"

# 区市町村の境界（国土数値情報 行政区域 N03、東京都）
N03_URL="https://nlftp.mlit.go.jp/ksj/gml/data/N03/N03-2025/N03-20250101_13_GML.zip"

CACHE_DIR="${ROOT}/.odpt-cache"

# fetch_keyed <URL（キーを除く）> <キー> <保存先>
# キーを含む URL を curl の引数に置くとプロセスの一覧に出るので、設定として標準入力から渡す。
# 失敗しても URL は表示しない（curl --show-error は URL を出さない）。
fetch_keyed() {
    case "$1" in
        *\?*) sep="&" ;;
        *) sep="?" ;;
    esac

    printf 'url = "%s%sacl:consumerKey=%s"\n' "$1" "${sep}" "$2" |
        curl --config - --fail --location --silent --show-error -o "$3"
}

update_extra() {
    if [ -f "${ROOT}/.env" ]; then
        # shellcheck disable=SC1091
        . "${ROOT}/.env"
    fi

    if [ -z "${ODPT_CONSUMER_KEY:-}" ] || [ -z "${ODPT_CHALLENGE_CONSUMER_KEY:-}" ]; then
        echo "Skipping other operators: set ODPT_CONSUMER_KEY and ODPT_CHALLENGE_CONSUMER_KEY in backend/.env"
        return
    fi

    rm -rf "${CACHE_DIR}/operators" "${CACHE_DIR}/dumps"

    for host in basic challenge; do
        if [ "${host}" = basic ]; then
            base="https://api.odpt.org/api/v4"
            key="${ODPT_CONSUMER_KEY}"
            operators="${BASIC_OPERATORS}"
        else
            base="https://api-challenge.odpt.org/api/v4"
            key="${ODPT_CHALLENGE_CONSUMER_KEY}"
            operators="${CHALLENGE_OPERATORS}"
        fi

        for op in ${operators}; do
            mkdir -p "${CACHE_DIR}/operators/${op}"
            for type in Railway Station TrainType; do
                echo "Downloading ${op} ${type}..."
                fetch_keyed "${base}/odpt:${type}?odpt:operator=odpt.Operator:${op}" "${key}" \
                    "${CACHE_DIR}/operators/${op}/${type}.json"
            done
        done

        # 時刻表は事業者ごとの API だと 1,000 件で切れるので、全件ダウンロード用の URL から取る
        mkdir -p "${CACHE_DIR}/dumps/${host}"
        for type in StationTimetable TrainTimetable; do
            echo "Downloading ${host} ${type} (all operators)..."
            fetch_keyed "${base}/odpt:${type}.json" "${key}" "${CACHE_DIR}/dumps/${host}/${type}.json"
        done
    done

    if [ ! -f "${CACHE_DIR}/n03/boundaries.geojson" ]; then
        echo "Downloading municipal boundaries (N03)..."
        mkdir -p "${CACHE_DIR}/n03"
        curl --fail --location --silent --show-error "${N03_URL}" -o "${CACHE_DIR}/n03/n03.zip"
        unzip -o -q "${CACHE_DIR}/n03/n03.zip" -d "${CACHE_DIR}/n03/unzipped"
        mv "$(find "${CACHE_DIR}/n03/unzipped" -name '*.geojson' | head -n 1)" "${CACHE_DIR}/n03/boundaries.geojson"
        rm -rf "${CACHE_DIR}/n03/n03.zip" "${CACHE_DIR}/n03/unzipped"
    fi

    station_files="${ASSETS_DIR}/station.json"
    for f in "${CACHE_DIR}"/operators/*/Station.json; do
        station_files="${station_files},${f}"
    done

    cd "${ROOT}"

    go run ./cmd/gen-assets -kind area \
        -geojson "${CACHE_DIR}/n03/boundaries.geojson" \
        -stations "${station_files}" \
        -out "${ASSETS_DIR}/tokyo_stations.txt"

    go run ./cmd/gen-assets -kind extra \
        -raw "${CACHE_DIR}" \
        -area "${ASSETS_DIR}/tokyo_stations.txt" \
        -out "${ASSETS_DIR}/extra"

    echo
    echo "Other operators' assets updated in assets/extra (do not commit them)."
}

if [ "${EXTRA_ONLY}" = false ]; then
    update_toei
fi

update_extra

ls -lh "${ASSETS_DIR}" "${ASSETS_DIR}/extra"
