# Train Status App

東京都交通局が公開するオープンデータを利用した鉄道運行情報Webアプリです。

運行情報や列車位置などのリアルタイムデータと、駅時刻表・運賃・乗降者数などの静的データを取得し、各路線・駅の情報を閲覧できます。

以下の路線に対応しています。

- 都営浅草線
- 都営三田線
- 都営新宿線
- 都営大江戸線
- 東京さくらトラム（都電荒川線）
- 日暮里・舎人ライナー

本プロジェクトは、AWSのサーバーレスアーキテクチャを採用し、TerraformによるInfrastructure as Code（IaC）、GoによるREST API、React + TypeScriptによるSPAとして構築しています。

---

# 主な機能

- 運行情報一覧
- 路線一覧
- 路線ごとの駅一覧
- 駅詳細
  - 時刻表（方面・平日・土休日別）
  - 乗降者数
- 列車現在位置検索
- 運賃検索

---

# システム構成

```text
Browser
   │
CloudFront
   ├── S3 (Frontend)
   └── API Gateway
            │
         AWS Lambda
      ├── 東京都交通局オープンデータAPI
      └── Static Assets (JSON)
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
- Zod

## Backend

- Go
- net/http

## Infrastructure

- AWS Lambda
- Amazon API Gateway
- Amazon CloudFront
- Amazon S3
- Terraform

---

# 設計方針

- サーバーレスアーキテクチャを採用
- TerraformによるInfrastructure as Code（IaC）
- Handler / Service / Client の責務分離
- Service層でデータの加工・集約を実施
- Go Genericsを利用した共通処理の抽象化
- TypeScript + Zodによる型安全なデータ管理
- 将来的なGTFS / GTFS Realtimeへの拡張を考慮した設計

---

# API

| Method | Endpoint | Description |
|---------|----------|-------------|
| GET | `/api/status` | 運行情報一覧 |
| GET | `/api/routes` | 路線一覧 |
| GET | `/api/routes/{routeId}/stations` | 路線ごとの駅一覧 |
| GET | `/api/stations/{stationId}` | 駅詳細（時刻表・乗降者数） |
| GET | `/api/trains/{trainNumber}/location` | 列車現在位置 |
| GET | `/api/fares?from={fromStation}&to={toStation}` | 運賃検索 |

---

# ライセンス

本アプリは東京都交通局が提供するオープンデータを利用しています。

このアプリでは東京都交通局が提供するオープンデータを加工して利用しています。

**提供者**

- 東京都交通局

**利用データ**

- 運行情報
- 路線情報
- 駅情報
- 列車ロケーション情報
- 駅時刻表
- 列車時刻表
- 運賃情報
- 乗降者数情報

**ライセンス**

Creative Commons Attribution 4.0 International（CC BY 4.0）

<https://creativecommons.org/licenses/by/4.0/deed.ja>