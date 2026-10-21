# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 概要

NORIKAE AI（非公式）: 東京都内の鉄道（都営は ODPT の `api-public.odpt.org`、APIキー不要。他社はキーの要る ODPT の API）のオープンデータを使った、運行情報・列車位置・時刻表・遅れを反映した経路検索・AI チャットのアプリ。リポジトリ名（`train-status-app`）は AWS のリソース名に使っているので変えない。
Go の REST API（AWS Lambda）と React + TypeScript の SPA（S3 + CloudFront）で構成し、インフラは Terraform で管理する。

領域ごとの詳しい説明は、各ディレクトリの CLAUDE.md にある（そのディレクトリのファイルを読むと読み込まれる）。作業の前に、関係する領域のものを読む。

- `backend/CLAUDE.md`: Go の層構成、静的データの更新、Swagger、AI の評価セット、ODPT データの注意点
- `frontend/CLAUDE.md`: React の構成、E2E テスト
- `infra/CLAUDE.md`: AWS の構成、CD・bootstrap の詳細、SSM、本番 Lambda の調査、Ruleset
- 設計: `docs/design/`（`route-search.md`・`ai-api.md`・`login.md`・`multi-operator.md`）

## 守ること

- AWS に反映される操作（`*-deploy`・`tf-*-apply`・`backend-extra-upload`・SSM への登録）は、実行前にユーザーへ確認する。手元で続けて実行するときは make を1つずつ順に実行し、前のものが成功したのを確かめてから次に進む（同時に実行すると、途中で止まったときに片方だけ反映される）
- API キー（Gemini・ODPT）と、AI への入力・応答の本文を、画面・会話・ログ・レスポンス・Terraform・tfvars・Lambda の環境変数に出さない。キーは SSM の SecureString に置き、ユーザーに `! aws ssm put-parameter ...` で登録してもらう（手順は `infra/CLAUDE.md`）。ODPT のキーは URL のクエリに入るので、キーを入れた URL や `*url.Error` もそのまま出さない。`backend/.env` の変数名を見るときは `sed 's/=.*/=<hidden>/' backend/.env`
- 都営以外の事業者のデータは、JSON も gob もコミットしない（ODPT のライセンスの第8条4項が、データと派生データを第三者が再利用できる状態で公開することを禁じている。このリポジトリは公開で、一度入れると履歴から消すのに force push が要り、main では禁止している）。`backend/assets/extra/` は gitignore 済み
- 列車は列車番号ではなく列車ID で特定する（列車番号は路線間・ダイヤ間で重複する）。運行日は3時前を前日として扱い、24時以降の時刻がある
- バックエンドの DTO（`backend/internal/service/service.go`）を変えたら、フロントの `src/types.ts`・`src/api.ts`・E2E の `e2e/data.ts` と、Swagger（`backend/docs/`）も合わせる。デプロイは `backend-deploy` → `tf-main-apply` → `frontend-deploy` まで行う（片方だけだと本番で型がずれる）
- deploy 用ロールには、Lambda の実行ロールの `iam:PassRole` 以外の IAM の権限を与えない。Lambda の実行ロールとポリシーは `infra/bootstrap` で管理する

## コマンド

ルートの `Makefile` に集約されている。

```bash
make up                 # docker compose でフロント(:5173)・バック(:8080)を起動
make backend-run        # cd backend && go run ./cmd/api
make backend-test       # go test ./...
make backend-vet        # go vet ./...
make backend-generate   # 埋め込み用の軽量データを再生成
make frontend-dev       # vite dev server（/api は BACKEND_URL か localhost:8080 にプロキシ）
make frontend-lint      # eslint
make frontend-build     # tsc -b && vite build（型チェックも兼ねる）
make frontend-e2e       # Playwright の E2E テスト（初回は cd frontend && npx playwright install chromium）
make backend-deploy / tf-main-plan / tf-main-apply / frontend-deploy   # 詳細は infra/CLAUDE.md
```

単一テストの実行:

```bash
cd backend && go test ./internal/service -run TestGetTrainLocation -v
```

テスト・vet・lint・build は、サブエージェントの `test-runner`（`.claude/agents/test-runner.md`、Haiku で動く。実行と結果の報告だけを行い、コードは直さない）に実行させる。失敗の調査と修正はメインの会話で行う。数秒で終わる単一テストは直接実行してよい。

画面は `verify-ui` スキルの手順で、手元で起動して Playwright MCP で確かめる。スクリーンショットはリポジトリの `.playwright-mcp/`（gitignore 済み）にしか保存できない。

## デプロイ

main へのマージで CI（`.github/workflows/ci.yml`）が、検証が通ったあとに `deploy.yml` を呼んで自動でデプロイする（変更のあった領域だけ）。全領域をやり直すときは Actions から CI を main で手動実行する。手元の make は、CD が失敗したときや bootstrap の適用に使う。`infra/bootstrap` を変えた PR は、マージの前に手元から `make tf-bootstrap-apply` で適用する。詳細は `infra/CLAUDE.md`。

## Git / PR

ブランチを切る・PR を作る前に `git fetch` と `gh pr list --state all` で main と PR の状態を確認する（`gh` は導入済み）。

PR では `ci.yml` が変更のあった領域だけを検証する（backend: gofmt・vet・test・Swagger が最新か / frontend: lint・build・E2E / infra: terraform fmt・validate と、PR への `terraform plan` の結果のコメント）。main は Ruleset で保護していて、ci.yml のジョブ名を変えたり検証のジョブを足したりしたら Ruleset の必須チェックも変える（`infra/CLAUDE.md`）。

`gh pr edit` は Projects (classic) 廃止の GraphQL エラーで失敗する。PR の題名・説明は `gh api -X PATCH repos/JulySeptember/train-status-app/pulls/<番号> -f title=... -F body=@<ファイル>` で更新する。

PR を作ったら `pr-review` スキル（`.claude/skills/pr-review/SKILL.md`）の手順でレビューさせる。Claude がマージするのは、ユーザーにマージを指示されたときだけ。レビューが通っただけではマージしない。
