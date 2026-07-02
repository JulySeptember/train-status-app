# 駅運行情報 Webアプリ 設計書

---

# 概要

東京都交通局が公開するオープンデータを利用し、都営地下鉄・東京さくらトラム（都電荒川線）・日暮里・舎人ライナーの運行情報をリアルタイムで表示するサーバーレスWebアプリ。

AWSのサーバーレスサービスを利用し、TerraformによるIaC、GoによるREST API、React + TypeScriptによるSPAとして構築する。

保守性・拡張性・型安全性を重視し、実務を意識した設計を採用する。

---

# システム構成

```
                    Browser
                       │
                       ▼
              CloudFront (HTTPS)
                /          \
               /            \
              ▼              ▼
      S3 (Frontend)     API Gateway
                              │
                              ▼
                       Lambda (Go)
                              │
                              ▼
            東京都交通局 オープンデータ(JSON)
```

CloudFrontをアプリケーション全体の入口とする。

- `/`
- `/assets/*`

はS3へ

- `/api/*`

はAPI Gatewayへルーティングする。

ブラウザからは同一オリジンとなるため、CORS設定は不要。

---

# アーキテクチャ

```
React
   │
CloudFront
   │
├── S3
└── API Gateway
        │
     Lambda
        │
東京都交通局 オープンデータ
```

---

# 採用技術

| Layer | Technology |
|--------|------------|
| Frontend | React 19 |
| Build Tool | Vite |
| Language | TypeScript |
| Styling | Tailwind CSS v4 |
| UI | shadcn/ui |
| Routing | React Router |
| API State | TanStack Query |
| Validation | Zod |
| Backend | Go |
| HTTP | net/http |
| Infrastructure | Terraform |
| Hosting | Amazon S3 |
| CDN | CloudFront |
| API | API Gateway HTTP API |
| Compute | AWS Lambda |
| CI/CD | Makefile |
| Local Development | Docker Compose |

---

# ディレクトリ構成

```
train-status-app/

├── frontend/
│   ├── src/
│   ├── package.json
│   ├── vite.config.ts
│   └── Dockerfile.dev
│
├── backend/
│   ├── cmd/
│   │   └── api/
│   │       └── main.go
│   │
│   ├── internal/
│   │   ├── client/
│   │   ├── config/
│   │   ├── handler/
│   │   ├── logger/
│   │   ├── middleware/
│   │   ├── model/
│   │   ├── router/
│   │   └── service/
│   │
│   ├── go.mod
│   ├── go.mod
│   └── Dockerfile.dev
│
├── infra/
│   ├── bootstrap/
│   │   ├── provider.tf
│   │   ├── s3.tf
│   │   ├── dynamodb.tf
│   │   └── backend.tf
│   │
│   ├── main/
│   │   ├── frontend_s3.tf
│   │   ├── backend_s3.tf
│   │   ├── cloudfront.tf
│   │   ├── api_gateway.tf
│   │   ├── lambda.tf
│   │   ├── iam.tf
│   │   ├── backend.tf
│   │   └── variables.tf
│   │
│   └── env/
│       └── dev.tfvars
│
├── docker-compose.yml
├── Makefile
└── README.md
```

---

# AWS構成

## Frontend

```
React

↓

S3

↓

CloudFront
```

### 役割

- SPA配信
- HTTPS
- CDN
- 静的ファイルキャッシュ

---

## Backend

```
Browser

↓

CloudFront

↓

API Gateway

↓

Lambda

↓

東京都交通局 オープンデータ
```

Lambdaはデータベースを持たず、

東京都交通局APIから取得したデータを加工して返却する。

---

# CloudFrontキャッシュ

CloudFrontはオリジンごとにキャッシュ設定を変更する。

### フロントエンド

```
JS
CSS
画像
フォント
```

長期間キャッシュ。

### API

```
/api/*
```

TTLを0秒とし、毎回Lambdaへ転送する。

これにより

- 静的ファイルは高速配信
- APIはリアルタイム取得

を両立する。

---

# Bootstrap

Terraform初回のみ実行。

作成するリソース

```
tfstate保存S3

Terraform Lock用DynamoDB

Lambda成果物保存S3
```

```
bootstrap

├── tfstate-bucket
├── terraform-lock-table
└── lambda-artifact-bucket
```

---

# Main Terraform

管理対象

- Frontend S3
- CloudFront
- API Gateway
- Lambda
- IAM
- CloudWatch Logs

---

# API一覧

---

## 運行情報

```http
GET /api/status
```

### 説明

都営地下鉄・東京さくらトラム・日暮里・舎人ライナーの運行情報一覧を取得します。

---

## 路線一覧

```http
GET /api/routes
```

### 説明

利用可能な路線一覧を取得します。

### レスポンス例

```json
[
  {
    "id": "asakusa",
    "name": "都営浅草線"
  },
  {
    "id": "mita",
    "name": "都営三田線"
  },
  {
    "id": "shinjuku",
    "name": "都営新宿線"
  },
  {
    "id": "oedo",
    "name": "都営大江戸線"
  },
  {
    "id": "arakawa",
    "name": "東京さくらトラム"
  },
  {
    "id": "nippori-toneri",
    "name": "日暮里・舎人ライナー"
  }
]
```

---

## 路線ごとの駅一覧

```http
GET /api/routes/{routeId}/stations
```

### パスパラメータ

| Name | Description |
|------|-------------|
| routeId | 路線ID |

### 例

```http
GET /api/routes/asakusa/stations
```

---

## 路線ごとの列車ロケーション情報

```http
GET /api/routes/{routeId}/locations
```

### パスパラメータ

| Name | Description |
|------|-------------|
| routeId | 路線ID |

### 例

```http
GET /api/routes/oedo/locations
```

---

## 駅時刻表

```http
GET /api/stations/{stationId}/timetable
```

### パスパラメータ

| Name | Description |
|------|-------------|
| stationId | 駅ID |

### 例

```http
GET /api/stations/shimbashi/timetable
```

---

## 列車時刻表

```http
GET /api/trains/{trainNumber}/timetable
```

### パスパラメータ

| Name | Description |
|------|-------------|
| trainNumber | 列車番号 |

### 例

```http
GET /api/trains/1001A/timetable
```

---

## 運賃情報

```http
GET /api/fares
```

### クエリパラメータ

| Name | Required | Description |
|------|----------|-------------|
| from | Yes | 出発駅ID |
| to | Yes | 到着駅ID |

### 例

```http
GET /api/fares?from=shimbashi&to=nishi-magome
```

---

## 駅別乗降者数

```http
GET /api/stations/{stationId}/passengers
```

### パスパラメータ

| Name | Description |
|------|-------------|
| stationId | 駅ID |

### 例

```http
GET /api/stations/shimbashi/passengers
```

---

# 利用する東京都交通局オープンデータ

無料・認証不要で利用可能なJSONデータを利用する。

- 運行情報
- 駅情報
- 路線情報
- 列車ロケーション情報
- 駅時刻表
- 列車時刻表
- 運賃情報
- 乗降者数情報

画像データ・PDFは利用対象外とする。

---

# 将来的な拡張

将来的にはGTFSおよびGTFS Realtimeにも対応できる設計とする。

### GTFS

- 路線情報
- 停車駅情報
- ダイヤ情報
- 便情報

### GTFS Realtime

- Trip Updates
- Vehicle Positions
- Service Alerts

現時点では実装対象外。

---

# Go設計

```
Handler

↓

Service

↓

Client

↓

東京都交通局API
```

## Client

HTTP通信のみ担当。

## Service

- Filter
- Map
- Sort
- データ加工
- レスポンス生成

副作用はClientへ閉じ込める。

---

# ジェネリクス

Go Genericsを積極的に利用する。

例

```
Fetch[T any]()
```

利用箇所

- JSONデコード
- APIレスポンス
- 共通ユーティリティ

---

# 関数型プログラミング

以下の考え方を取り入れる。

- 小さな関数
- 純粋関数優先
- 副作用を最小化
- データ変換を関数へ分離

```
取得

↓

Filter

↓

Map

↓

Sort

↓

Response
```

---

# フロントエンド設計

```
API

↓

Zod

↓

TypeScript

↓

React
```

APIレスポンスはunknownとして受け取り、

Zodで検証後に型へ変換する。

---

# 状態管理

サーバーデータ

- TanStack Query

ローカル状態

- useState
- Context API

必要になった場合のみZustandを導入する。

---

# CORSを不要にする構成

CloudFrontをアプリケーション入口に統一する。

```
example.com

↓

/

↓

S3

----------------

example.com

↓

/api/*

↓

API Gateway
```

ブラウザからは同一オリジンとなるため、

CORS設定は不要。

---

# CI/CD

Makefileを共通入口とする。

```
test

↓

frontend build

↓

backend build

↓

Lambda ZIP作成

↓

Artifact S3 Upload

↓

Terraform Apply

↓

Lambda Update
```

GitHub Actions導入時もMakefileをそのまま利用する。

---

# Docker

Docker Composeをローカル開発専用として利用する。

```
React

+

Go API
```

AWS環境の再現は目的としない。

---

# ライセンス表記

本アプリは東京都交通局が公開するオープンデータを加工して利用するため、CC BY 4.0に基づくクレジット表記を行う。

アプリ内の「ライセンス」または「このアプリについて」ページ、およびREADMEに以下を掲載する。

```
本アプリは東京都交通局が提供するオープンデータを利用しています。

このアプリは、以下の著作物を改変して利用しています。

提供者
東京都交通局

利用データ
・運行情報
・駅情報
・路線情報
・列車ロケーション情報
・駅時刻表
・列車時刻表
・運賃情報
・乗降者数情報

ライセンス
クリエイティブ・コモンズ 表示4.0 国際
https://creativecommons.org/licenses/by/4.0/deed.ja
```

フッターには

```
データ提供：東京都交通局（CC BY 4.0）
```

へのリンクを表示する。

---

# 設計方針

- フロントエンド・バックエンド完全分離
- CloudFrontを単一エントリーポイントとしCORS不要
- サーバーレスアーキテクチャ
- TerraformによるIaC
- MakefileをCI/CDの共通入口として利用
- Go Genericsを活用し重複コードを削減
- 関数型プログラミングの考え方を採用
- TypeScript + Zodによる型安全
- 東京都交通局の無料オープンデータ(JSON)を利用
- 将来的なGTFS・GTFS Realtime対応を考慮
- データベースを持たない軽量構成
- AWS無料利用枠でも運用しやすい構成