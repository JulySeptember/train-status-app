# NORIKAE AI

東京都内の鉄道の運行情報・列車位置・時刻表と、遅れを反映した乗り換え案内を、AI に話しかけて調べられる Web アプリです。公共交通オープンデータセンター（ODPT）を通じて各鉄道事業者が公開するオープンデータを利用しています。

> 本アプリは個人が作成した**非公式**のアプリで、各鉄道事業者・公共交通オープンデータセンターとは関係ありません。

# 🌍 Live Demo

| Service | URL |
| --- | --- |
| Application | https://d2ck0n8si4rgsn.cloudfront.net |
| Swagger UI | https://a2y4udf8gb.execute-api.ap-northeast-1.amazonaws.com/swagger/index.html |

---

運行情報や列車位置などの動的データと、駅時刻表・列車時刻表・乗降者数などの静的データを組み合わせ、各路線・駅の情報の閲覧、遅れを反映した経路検索、AI との会話による照会ができます。

# 対応路線

東京都内（島しょ部を除く）に駅のある、次の事業者の路線（設計は [docs/design/multi-operator.md](docs/design/multi-operator.md)）。

- 都営交通（浅草線・三田線・新宿線・大江戸線・東京さくらトラム・日暮里・舎人ライナー）
- 東京メトロ・JR東日本・京王電鉄・東武鉄道・りんかい線・つくばエクスプレス・多摩都市モノレール
- 東急電鉄・西武鉄道・小田急電鉄・京急電鉄・ゆりかもめ（駅の時刻表と運行情報のみ。経路検索は未対応。小田急・ゆりかもめは運行情報も無し）

都営以外の事業者のデータは、ライセンス（公共交通オープンデータ基本ライセンス・チャレンジ限定ライセンス）で再配布が禁じられているため、このリポジトリには含めていません。

本プロジェクトは、AWSのサーバーレスアーキテクチャを採用し、TerraformによるInfrastructure as Code（IaC）、GoによるREST API、React + TypeScriptによるSPAとして構築しています。

---

# 主な機能

- 運行情報一覧
- 路線一覧
- 路線ごとの駅一覧
- 駅詳細
  - 時刻表（方面・平日・土休日別、行先・列車種別つき）
  - 乗降者数
- 列車の現在位置（路線図上に表示。出発前・運行終了も案内）
- 経路検索
  - RAPTOR による乗り換え探索
  - 走行中の列車の遅れと、運行情報の見合わせを反映
- AI チャット（Gemini）
  - 「次の電車は？」「遅れてる？」のような自然な文章で、運行情報・列車位置・時刻表・経路を聞ける
  - AI が道具（照会 API）を呼び、アプリが結果を返す、を繰り返して答える
- 運賃検索
- SwaggerによるAPIドキュメント

---

# システム構成

```text
Browser
   │
CloudFront
   ├── Default (*)
   │       │
   │       └── Amazon S3
   │
   └── /api/*
           │
     Amazon API Gateway
           │
       AWS Lambda ──── Gemini API（AI チャット）
           │    ├── SSM Parameter Store（Gemini・ODPT のキー）
           │    └── DynamoDB（AI の利用回数）
           │
公共交通オープンデータセンター（ODPT）API
```

---

# 使用技術

## Frontend

- React 19
- TypeScript
- Vite
- Tailwind CSS v4
- shadcn/ui
- React Router
- TanStack Query

## Backend

- Go 
- net/http
- REST API
- JSON API
- swaggo (Swagger/OpenAPI)
- RAPTOR（経路探索）
- Gemini API（AI エージェント・Function Calling）

## Infrastructure

- Terraform
- AWS Lambda
- Amazon API Gateway (HTTP API)
- Amazon CloudFront
- Amazon S3
- Amazon DynamoDB（AI の利用上限の管理）
- AWS Systems Manager Parameter Store（API キーの保管）
- IAM
- GitHub Actions（CI/CD、OIDC で AWS に接続）

---

# 設計方針

- AWS Lambdaを利用したサーバーレスアーキテクチャ
- TerraformによるInfrastructure as Code（IaC）
- Handler / Service / Clientによる責務分離
- Service層でデータの加工・集約を実施
- Go Genericsを利用した共通処理の抽象化
- Docker Composeによるローカル開発環境の統一

---

# API

| Method | Endpoint | Description |
|---------|----------|-------------|
| GET | `/api/status` | 運行情報一覧 |
| GET | `/api/routes` | 路線一覧 |
| GET | `/api/routes/{routeId}/stations` | 路線ごとの駅一覧 |
| GET | `/api/stations/{stationId}` | 駅詳細（時刻表・乗降者数） |
| GET | `/api/trains/{trainId}/location` | 列車現在位置 |
| GET | `/api/fares?from={fromStation}&to={toStation}` | 運賃検索 |
| GET | `/api/journeys?from={fromStation}&to={toStation}` | 経路検索（遅れを反映） |
| POST | `/api/chat` | AI チャット |

詳細なAPI仕様はSwagger UIから確認できます。

---

# 開発環境

Docker Composeを利用したローカル開発環境を構築しています。

Frontend・Backendをコンテナ分離し、開発環境を統一しています。

```text
Docker Compose

frontend
 └── React + Vite
      :5173

backend
 └── Go REST API
      :8080
```

起動

```bash
make up
```

---

# ライセンス

本アプリは、公共交通オープンデータセンターを通じて各鉄道事業者が提供するオープンデータを加工して利用しています。情報の正確性・完全性は保証しません。

| 事業者 | ライセンス |
| --- | --- |
| 東京都交通局 | [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/deed.ja)（著作権は東京都交通局に帰属します） |
| 東京メトロ・東京臨海高速鉄道・首都圏新都市鉄道・多摩都市モノレール・ゆりかもめ | [公共交通オープンデータ基本ライセンス](https://developer.odpt.org/terms/data_basic_license.html) |
| JR東日本・京王電鉄・東武鉄道・京急電鉄・東急電鉄・西武鉄道・小田急電鉄 | [公共交通オープンデータチャレンジ限定ライセンス](https://developer.odpt.org/challenge_license)（公共交通オープンデータチャレンジ 2026 への応募のための利用。許諾は 2027年3月12日まで） |

**利用データ**

- 運行情報
- 路線情報
- 駅情報
- 列車ロケーション情報
- 駅時刻表
- 列車時刻表
- 運賃情報（都営のみ）
- 乗降者数情報（都営のみ）
