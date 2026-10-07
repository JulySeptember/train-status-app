# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 概要

東京都交通局（都営）のオープンデータ（ODPT API, `api-public.odpt.org`、APIキー不要）を使った鉄道運行情報アプリ。
Go の REST API（AWS Lambda）と React + TypeScript の SPA（S3 + CloudFront）で構成し、インフラは Terraform で管理する。

## コマンド

ルートの `Makefile` に集約されている。

```bash
make up                 # docker compose でフロント(:5173)・バック(:8080)を起動
make backend-run        # cd backend && go run ./cmd/api
make backend-test       # go test ./...
make backend-vet        # go vet ./...
make frontend-dev       # vite dev server（/api は BACKEND_URL か localhost:8080 にプロキシ）
make frontend-lint      # eslint
make frontend-build     # tsc -b && vite build（型チェックも兼ねる）
```

単一テストの実行:

```bash
cd backend && go test ./internal/service -run TestGetTrainLocation -v
```

フロントエンドにテストはない。

### Swagger

`backend/docs/` は swaggo が生成したファイル（手で編集しない）。handler のアノテーションや DTO を変えたら再生成する:

```bash
cd backend && swag init -g cmd/api/main.go -o docs
```

### 静的データの更新

`backend/scripts/update_assets.sh` が ODPT から `backend/assets/*.json` を取り直す。

### デプロイ（AWS に反映される操作。実行前にユーザーへ確認する）

- `make backend-deploy`: linux/arm64 でビルドして zip にし、S3 にアップロードする。Lambda への反映は `make tf-main-apply`（`s3_object_version` を参照している）
- `make frontend-deploy`: ビルドして S3 に sync し、CloudFront を invalidate する
- `make tf-main-plan` / `tf-main-apply`: `infra/main` を `infra/env/dev.tfvars` で適用する。`tf-*-apply` / `destroy` は `-auto-approve` 付き
- `infra/bootstrap`: tfstate 用 S3・DynamoDB と Lambda アーティファクト用 S3 を作る

## アーキテクチャ

### バックエンド（`backend/`, Go 1.25, 標準 `net/http`）

- `cmd/api/main.go`: `AWS_LAMBDA_RUNTIME_API` があれば `aws-lambda-go-api-proxy` の httpadapter（API Gateway HTTP API, payload v2）で動き、なければ通常の HTTP サーバとして動く。CORS はローカル実行時だけ有効
- 層構成は `router` → `handler` → `service` → `client`（ODPT へのリアルタイム取得）/ `assets`（静的データ）
  - `handler` は service のセンチネルエラー（`ErrStationNotFound`、`ErrTrainNotFound`、`ErrExternalAPI` など）を HTTP ステータスに変換する
  - `service` はデータの加工・集約を担い、`model`（ODPT JSON-LD そのままの型）をフロント向けの DTO に変換する。DTO は service.go に定義する
  - `client` で外部 API を呼ぶのは運行情報（`odpt:TrainInformation`）と列車位置（`odpt:Train`）だけ。共通処理はジェネリクスの `fetch[T]`
- `assets/`: ODPT の静的データ（路線・駅・運賃・駅時刻表・列車時刻表・乗降人員）を `go:embed` で埋め込み、起動時に全件 `json.Unmarshal` する。`station_timetable.json` は約25MBあり、Lambda のコールドスタートの主な要因になっている
- `internal/calendar`: 運行日（3時前は前日扱い）と、その日に適用されるダイヤ種別（`odpt.Calendar:*`）を判定する。祝日は祝日法に基づいて計算する
- テストは `service` と `calendar` にあり、service のテストは `mockClient` と実際の embed アセットを使う

### ODPT データの注意点

- 列車番号（`odpt:trainNumber`）は路線間で重複し、平日・土休日ダイヤでも使い回される。列車は必ず列車ID（`odpt.Train:Toei.<路線>.<番号>`）で特定する。service は起動時に駅時刻表から列車ID → 路線・列車番号の索引を作る
- 日暮里・舎人ライナーは `odpt:Train` が配信されない（`trainLocationUnsupported`）
- `odpt:Train` で `toStation` が null の列車は `fromStation` に停車中
- 路線によって「土曜・休日」が別ダイヤのものと「土休日」にまとめられたものがある（フロントの `Timetable.tsx` も両方に対応している）

### フロントエンド（`frontend/`）

- React 19 + Vite + Tailwind v4 + shadcn/ui（`components/ui`）、TanStack Query、React Router。パスエイリアス `@` は `src/` を指す
- API 呼び出しは `src/api.ts`、レスポンス型は `src/types.ts` に集約している。バックエンドの DTO を変えたらこの2ファイルも合わせる
- ODPT の ID を日本語ラベルに変換する処理は `src/lib/odpt.ts`
- ID は `odpt.Station:...` のように `:` や `.` を含むので、URL に入れるときは `encodeURIComponent` する

### インフラ（`infra/`）

CloudFront が `/api/*` を API Gateway（HTTP API）→ Lambda（`provided.al2023`, arm64, 256MB）に流し、それ以外を S3 に流す。リージョンは `ap-northeast-1`。
