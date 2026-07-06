# Train Status App

東京都交通局が公開するオープンデータを利用した駅運行情報Webアプリです。

運行情報・列車位置・駅時刻表・運賃・乗降者数などを取得し、リアルタイムな駅情報を閲覧できます。

以下の路線の運行情報・列車位置・駅情報・時刻表・運賃・乗降者数を閲覧できます。

- 都営浅草線
- 都営三田線
- 都営新宿線
- 都営大江戸線
- 東京さくらトラム（都電荒川線）
- 日暮里・舎人ライナー

本プロジェクトは、AWSのサーバーレスアーキテクチャを採用し、TerraformによるIaC、GoによるREST API、React + TypeScriptによるSPAとして構築しています。

---

# 主な機能

- 運行情報一覧
- 路線一覧
- 路線ごとの駅一覧
- 駅詳細
  - 時刻表
  - 乗降者数
- 列車現在位置
- 運賃検索

---

# システム構成

```
Browser
   │
CloudFront
   ├── S3 (Frontend)
   └── API Gateway
            │
         Lambda
      ├── 東京都交通局API
      └── Assets(JSON)
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
- Go Generics

## Infrastructure

- AWS Lambda
- API Gateway
- CloudFront
- Amazon S3
- Terraform

---

# 設計方針

- サーバーレスアーキテクチャを採用
- TerraformによるInfrastructure as Code
- Handler / Service / Client の責務分離
- Service層でデータ加工・集約を実施
- Go Genericsを利用し共通処理を抽象化
- TypeScript + Zodによる型安全
- 将来的なGTFS・GTFS Realtime対応を考慮

---

# API

| Method | Endpoint |
|---------|----------|
| GET | /api/status |
| GET | /api/routes |
| GET | /api/routes/{routeId}/stations |
| GET | /api/stations/{stationId} |
| GET | /api/trains/{trainNumber}/location |
| GET | /api/fares |

---

# ライセンス

本アプリは東京都交通局が提供するオープンデータを利用しています。

このアプリは東京都交通局が提供するオープンデータを加工して利用しています。

**提供者**

東京都交通局

**利用データ**

- 運行情報
- 駅情報
- 路線情報
- 列車ロケーション情報
- 駅時刻表
- 列車時刻表
- 運賃情報
- 乗降者数情報

**ライセンス**

Creative Commons Attribution 4.0 International (CC BY 4.0)

https://creativecommons.org/licenses/by/4.0/deed.ja