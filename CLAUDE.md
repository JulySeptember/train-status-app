# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 概要

NORIKAE AI（非公式）: 東京都内の鉄道（都営は ODPT の `api-public.odpt.org`、APIキー不要。他社はキーの要る ODPT の API）のオープンデータを使った、運行情報・列車位置・時刻表・遅れを反映した経路検索・AI チャットのアプリ。リポジトリ名（`train-status-app`）は AWS のリソース名に使っているので変えない。
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

テスト・vet・lint・build は、サブエージェントの `test-runner`（`.claude/agents/test-runner.md`、Haiku で動く。実行と結果の報告だけを行い、コードは直さない）に実行させる。失敗の調査と修正はメインの会話で行う。数秒で終わる単一テストは直接実行してよい。

フロントエンドにテストはない。
画面は手元で起動して Playwright MCP で確かめる。スクリーンショットはリポジトリの `.playwright-mcp/`（gitignore 済み）にしか保存できない。

AI エージェントの評価セット（`backend/internal/ai/testdata/eval.yaml`）は実際の Gemini を呼ぶので、手で実行する（CI では動かない。無料枠を数十回使い、約7分かかる）。キーは SSM から直接渡し、画面や会話に出さない:

```bash
cd backend && AI_EVAL=1 GEMINI_API_KEY="$(aws ssm get-parameter --region ap-northeast-1 --name /train-status-app/dev/gemini-api-key --with-decryption --query Parameter.Value --output text)" go test ./internal/ai -run 'TestEval$' -v -timeout 30m
```

### Swagger

`backend/docs/` は swaggo が生成したファイル（手で編集しない）。handler のアノテーションや DTO を変えたら再生成する:

```bash
cd backend && swag init -g cmd/api/main.go -o docs
```

手元の `swag` が go.mod の swaggo/swag と違うバージョンだと CI で差分が出る。`go run github.com/swaggo/swag/cmd/swag@<go.mod のバージョン> init -g cmd/api/main.go -o docs` で実行する。

### 静的データの更新

`backend/scripts/update_assets.sh` が ODPT から `backend/assets/*.json` を取り直す。`station_timetable.json` / `train_timetable.json` を更新したら、埋め込み用の軽量データを再生成する（忘れると `assets` のテストが失敗する）:

```bash
make backend-generate   # = cd backend && go generate ./assets
```

同じスクリプトが、`backend/.env` のキーがあれば都営以外の事業者のデータも取り、都内に絞って `backend/assets/extra/`（gitignore 済み。コミットしない）に置く。他社のデータだけを取り直すときは `sh scripts/update_assets.sh --extra-only`。取得した元の JSON（約370MB）と区市町村の境界は `backend/.odpt-cache/` に残る。都内の駅の一覧 `assets/tokyo_stations.txt`（駅IDだけなのでコミットする）もこのとき作り直す。`extra/` のデータは `assets.New(assets.WithExtra())` で読み込み、無ければ都営だけで動く（CI のテスト）。

本番に入れる `extra/` は、非公開の S3（Lambda アーティファクト用のバケットの `assets-extra/<版>/`）に置き、使う版を `backend/assets/extra.version`（版の名前だけなのでコミットする）に書く。他社のデータを取り直したら `make backend-extra-upload`（S3 へのアップロード。AWS に反映される操作なので実行前にユーザーへ確認する）で版を作り、`extra.version` をコミットする。`make backend-build` は `extra/` がそろっていなければ失敗し、`make backend-deploy` は先に `make backend-extra-download` で `extra.version` の版を取ってくる（手元の `extra/` は上書きされる）。

### デプロイ（AWS に反映される操作。実行前にユーザーへ確認する）

main へのマージで CI（`.github/workflows/ci.yml`）が、検証が通ったあとに `deploy.yml` を呼んで自動でデプロイする。変更のあった領域だけを backend → `terraform apply` → frontend の順に反映する。全領域をやり直すときは Actions から CI を main で手動実行する（`workflow_dispatch`）。手元の make は、CD が失敗したときや bootstrap の適用に使う。

main への push が続くと、待機中の実行は新しいものに置き換わり、その push の変更が反映されないことがある（変更の判定は push 単位のため）。その場合も手動実行で反映する。

`deploy.yml` のジョブに GitHub Environment（`environment:`）を付けると、OIDC の `sub` が `repo:<owner>/<repo>:environment:<名前>` に変わり、deploy 用ロールを引き受けられなくなる。付けるときは `infra/bootstrap/github_oidc.tf` の条件も変える。

- `make backend-deploy`: linux/arm64 でビルドして zip にし、S3 にアップロードする。Lambda への反映は `make tf-main-apply`（`s3_object_version` を参照している）
- `make frontend-deploy`: ビルドして S3 に sync し、CloudFront を invalidate する
- `make tf-main-plan` / `tf-main-apply`: `infra/main` を `infra/env/dev.tfvars` で適用する。`tf-*-apply` / `destroy` は `-auto-approve` 付き
- `infra/bootstrap`: tfstate 用 S3・DynamoDB、Lambda アーティファクト用 S3、GitHub Actions 用の OIDC プロバイダと IAM ロール（PR の plan 用・main のデプロイ用）、Lambda の実行ロール（`lambda_role.tf`）とそれに付けるポリシー（`lambda_ai.tf`・`lambda_odpt.tf`）を作る。state はローカルにあるので、手元から `make tf-bootstrap-apply` で適用する。bootstrap を変えた PR は、マージ（CD の `terraform apply`）の前に bootstrap を適用する
- Lambda の実行ロールとそれに付けるポリシーは `infra/bootstrap`（`lambda_role.tf`）で作る。権限を足すときは、ポリシーを作って `aws_iam_role_policy_attachment.lambda` に足す。deploy 用ロールには、そのロールの `iam:PassRole` 以外の IAM の権限を与えない（信頼ポリシーやポリシーを変えられると、SSM の API キーを読めるロールを引き受けたり、権限を広げたりできる）

手元でデプロイするときは、make を1つずつ順に実行し、前のコマンドが成功したのを確かめてから次に進む（同時に実行すると、途中で止まったときに片方だけ反映される）。

バックエンドの DTO（`service.go`）とフロントの `src/types.ts` を合わせて変えた変更は、`backend-deploy` → `tf-main-apply` のあとに `frontend-deploy` も行う。片方だけ反映すると本番でフロントとバックの型がずれる。

### 本番 Lambda の調査（読み取りのみ）

```bash
aws logs filter-log-events --region ap-northeast-1 \
  --log-group-name /aws/lambda/train-status-app-dev-api \
  --filter-pattern '"REPORT"'   # Init Duration / Max Memory Used を確認
```

### Git / PR

ブランチを切る・PR を作る前に `git fetch` と `gh pr list --state all` で main と PR の状態を確認する（`gh` は導入済み）。

PR では `.github/workflows/ci.yml` が変更のあった領域だけを検証する（backend: gofmt・vet・test・Swagger が最新か / frontend: lint・build / infra: terraform fmt・validate と、PR への `terraform plan` の結果のコメント）。Swagger のチェックは go.mod の swaggo/swag と同じバージョンの CLI で再生成して差分を見る。

main は Ruleset（`main`）で保護している: PR 必須（承認は不要）、force push・削除の禁止、必須チェックは `changes`・`backend`・`frontend`・`infra`（ci.yml のジョブ名）。`if` でスキップされたジョブは成功として扱われるので、変更のない領域があってもマージできる。`changes` が失敗すると後続がスキップされて通ってしまうため、`changes` も必須にしている。ci.yml のジョブ名を変えたり検証のジョブを足したりしたら、Ruleset の必須チェックも合わせて変える（`gh api repos/JulySeptember/train-status-app/rulesets`）。`infra-plan` は AWS 側の一時的な失敗やフォークからの PR があるので必須にしていない。

`gh pr edit` は Projects (classic) 廃止の GraphQL エラーで失敗する。PR の題名・説明は `gh api -X PATCH repos/JulySeptember/train-status-app/pulls/<番号> -f title=... -F body=@<ファイル>` で更新する。

### PR のレビュー

PR を作ったら、実装時の会話を持たないサブエージェント（Agent ツールの general-purpose）にレビューさせる。渡すのは PR の番号と説明（意図）と下の観点だけにし、実装の経緯や「ここは問題ない」といった判断は渡さない。差分は `gh pr diff <番号>` で読ませ、周りのコードは必要に応じて読ませる。

- 省略してよいもの: ドキュメント（`*.md`）だけの変更、`backend/assets` のデータの更新だけの変更
- 観点は正しさ（バグ、境界値、エラー処理）、セキュリティ、テストの欠落に絞る。命名や整形などの書き方は指摘させない
- このリポジトリで特に見させる点:
  - API キーや AI への入力・応答の本文が、ログ・レスポンス・Terraform・tfvars・Lambda の環境変数に出ていないか
  - deploy 用ロールに IAM を変える権限（ロール・信頼ポリシー・ポリシーの付け外し）を与えていないか（Lambda の実行ロールは `infra/bootstrap` で管理する）
  - 列車を列車番号ではなく列車ID で特定しているか
  - 運行日（3時前は前日扱い）と、24時以降の時刻の扱い
  - DTO（`service.go`）と `src/types.ts`、handler のアノテーションと `backend/docs/` がずれていないか
  - 常駐するデータ（`model.StationTimetableEntry` など）に使わない項目を足していないか
- 出力は重大度（High / Medium / Low）付きの指摘の一覧にさせる。確信が持てないものは「要確認」と書かせる
- High は直して push し、新しいサブエージェントに同じ条件で再レビューさせる。再レビューは2回まで。それでも残る指摘と、直さなかった Medium 以下の指摘は、理由を添えてユーザーに伝える
- Claude がマージするのは、ユーザーにマージを指示されたときだけ。レビューが通っただけではマージしない

## アーキテクチャ

### バックエンド（`backend/`, Go 1.25, 標準 `net/http`）

- `cmd/api/main.go`: `AWS_LAMBDA_RUNTIME_API` があれば `aws-lambda-go-api-proxy` の httpadapter（API Gateway HTTP API, payload v2）で動き、なければ通常の HTTP サーバとして動く。CORS はローカル実行時だけ有効
- 層構成は `router` → `handler` → `service` → `client`（ODPT へのリアルタイム取得）/ `assets`（静的データ）
  - `handler` は service のセンチネルエラー（`ErrStationNotFound`、`ErrTrainNotFound`、`ErrExternalAPI` など）を HTTP ステータスに変換する
  - `service` はデータの加工・集約を担い、`model`（ODPT JSON-LD そのままの型）をフロント向けの DTO に変換する。DTO は service.go に定義する
  - `client` で外部 API を呼ぶのは運行情報（`odpt:TrainInformation`）と列車位置（`odpt:Train`）だけ。共通処理はジェネリクスの `fetch[T]`。事業者ごと（`client.Sources`）に並列に取り、一部が失敗しても残りを返す（全部失敗したときだけ `ErrExternalAPI`）。都営以外の事業者は環境変数 `ODPT_OPERATORS`（tfvars の `odpt_operators`）に書いたものだけを取る。キーは手元では `ODPT_CONSUMER_KEY`・`ODPT_CHALLENGE_CONSUMER_KEY`（`backend/.env`）、Lambda では SSM から読む（`internal/odptkey`）。キーは URL に入るので、`*url.Error` をそのままログやエラーに出さない
- `assets/`: ODPT の静的データ（路線・駅・運賃・駅時刻表・列車時刻表・乗降人員・列車種別）を `go:embed` で埋め込み、起動時に全件読み込む。駅時刻表と列車時刻表は、それぞれ約25MBの JSON を埋め込まず、使う項目に絞って文字列表にまとめた `station_timetable.gob` / `train_timetable.gob`（`assets/slim`、`cmd/gen-assets` で生成）を埋め込む。Lambda のコールドスタートを短くするため。`model.StationTimetableEntry` も約12万件が常駐するので、使う項目以外を足さない。列車時刻表は経路探索用で、`model` の型に戻さず、文字列表の番号と分（3時前は +24時間）のまま `slim.TrainTimetables` で持つ
- `internal/route`: 経路探索エンジン（RAPTOR）。遅れは `Query.Delays`（路線・方向ごと）で受け取り、パターンの時刻に足して探す（`shift`）。service（`service/realtime.go`）が `odpt:Train` の遅れの中央値と、運行情報の文章による見合わせの判定を30秒キャッシュして渡す。`GET /api/journeys` から使う。`slim.TrainTimetables` を数値のまま使い、駅・路線は ID の文字列で扱う（都営に決め打ちしない）。乗り換えの対応表（`route.Transfer`）は service（`service/transfer.go`）が作って渡す: 600m 以内の同じ名前の駅どうし、他社の駅データの `odpt:connectingStation`（逆向きも足す）、都営の名前が違う駅の組（東日本橋 ⇔ 馬喰横山）。時間は駅の座標の距離から決める。600m 以内の同じ名前の駅は、出発駅・到着駅としては1つの駅にまとめ、出発駅・到着駅から対応表で歩ける駅でも乗り降りする（歩く時間は経路の出発・到着の時刻に含める）。設計は `docs/design/route-search.md`
- `internal/ai`: AI エージェント（`POST /api/chat`）。AI が道具（`tools.go`）を呼び、アプリが service の照会（`service/assistant.go`）を実行して結果を返す、を最大5往復繰り返す。Provider は Gemini（`ai/gemini`、REST を直接呼ぶ）とテスト用の `ai/fake`。駅名の特定は `internal/station`。API キーが無いとき（手元で `GEMINI_API_KEY` を設定していないとき、Lambda では SSM の準備ができるまで）は `/api/chat` が 503 を返す。ログに入力・応答の本文を残さない。設計は `docs/design/ai-api.md`
- `internal/calendar`: 運行日（3時前は前日扱い）と、その日に適用されるダイヤ種別（`odpt.Calendar:*`）を判定する。祝日は祝日法に基づいて計算する
- テストは `service`・`route`・`handler`・`calendar`・`assets`・`ai`・`station` にある。service のテストは `mockClient` と実際の embed アセットを使う。route のテストは小さな架空の路線網で確かめる

### ODPT データの注意点

- 列車番号（`odpt:trainNumber`）は路線間で重複し、平日・土休日ダイヤでも使い回される。列車は必ず列車ID（`odpt.Train:Toei.<路線>.<番号>`）で特定する。service は起動時に駅時刻表から列車ID → 路線・列車番号の索引を作る
- 東急・西武・小田急・京急・ゆりかもめは列車時刻表が無く、駅時刻表にも列車ID・到着時刻が無い。データ生成（`cmd/gen-assets -kind extra`）で、駅時刻表の発車をつないで列車を推定して列車時刻表に足す（`internal/infer`。列車ID に `.Estimated.` が入り、列車番号は空）。推定の正しさは列車時刻表のある事業者で測っている（`internal/infer/accuracy_test.go`）
- 直通運転の列車は、事業者（路線）ごとに別の列車として載っていて、`odpt:nextTrainTimetable` も事業者をまたがない。列車番号も事業者ごとに付け方が違う（JR 1016K ⇔ メトロ A1017K）。経路探索は境目の駅で行先と時刻からつなぐ（`internal/route/through.go`。結果の区間は `through`）
- 日暮里・舎人ライナーは `odpt:Train` が配信されない（`trainLocationUnsupported`）。荒川線は `odpt:Train` は配信されるが `odpt:delay` が null（`model` では 0 になる）。他社は、`odpt:Train` を配信している事業者を `client.Sources` の `Location` で表す（メトロなどは配信していない。京急は `odpt:delay` が null）
- `odpt:Train` で `toStation` が null の列車は `fromStation` に停車中
- `odpt:Train` は走り出した列車しか配信しない。始発駅で発車を待つ列車（浅草線の泉岳寺始発など）は、時刻表の次発でも位置が無い。位置の無い列車の状態（出発前・運行終了）は、本日の列車時刻表から判定している（`service/train_schedule.go`）
- 路線によって「土曜・休日」が別ダイヤのものと「土休日」にまとめられたものがある（フロントの `Timetable.tsx` も両方に対応している）
- 時刻表の行先には直通運転先（京急・京成・東急など他社）の駅が含まれるが、他社の駅データは公開 API から取れない。駅名は `service/through_service.go` の辞書で引く（`assets/extra` があれば、都外の行先駅は `extra/destination_station.json` から引く）。assets 更新後に起動ログへ `unknown destination station` が出たら辞書に追加する（辞書には ODPT にデータの無い事業者の駅だけを書く）
- 都営の駅データ（`station.json`）には乗り換え先の情報（`odpt:connectingStation`）がない（他社の駅データにはある）。都営どうしの名前が違う乗り換え駅は `service/transfer.go` の `differentNameTransfers` に手で足す
- 大江戸線の環状部（外回り・内回り）は、駅時刻表に行先（`odpt:destinationStation`）が入っていない
- ODPT 公開 API は1回の取得で最大1,000件しか返さない。`railway_fare.json` はちょうど1,000件で、途中までしか取れていない。列車時刻表は全件ダウンロード用の URL（`odpt:TrainTimetable.json`、リダイレクトされる）から取っている
- ODPT のデータの一覧は `https://ckan.odpt.org/dataset/?_organization_limit=0&tags=%E9%89%84%E9%81%93-railway`（鉄道）。CKAN の API は使えないので HTML から読む。都営は JSON のほかに GTFS / GTFS-RT もあるが使っていない（理由は `docs/design/route-search.md` 3章）
- 都営以外の事業者のデータは、キーの要る `api.odpt.org`（公共交通オープンデータ基本ライセンス）と `api-challenge.odpt.org`（チャレンジ限定ライセンス）から取る。どちらのライセンスも第8条4項で、データと派生データ（gob のように元のデータを復元できるもの）を第三者が再利用できる状態で公開することを禁じている。このリポジトリは公開なので、**他社のデータは JSON も gob もコミットしない**（一度入れると履歴から消すのに force push が要り、main では禁止している）。今の `backend/assets` は都営（CC BY 4.0）だけなので置いてよい。置き場所は `docs/design/multi-operator.md` 4章
- ODPT のキーは手元では `backend/.env`（gitignore 済み）の `ODPT_CONSUMER_KEY`（`api.odpt.org`）・`ODPT_CHALLENGE_CONSUMER_KEY`（`api-challenge.odpt.org`）。キーは URL のクエリ（`acl:consumerKey=`）に入るので、`.env` の中身や、キーを入れた URL を画面・会話・ログに出さない（`.env` の変数名を見るときは `sed 's/=.*/=<hidden>/' backend/.env`）
- 事業者ごとの API は列車時刻表・駅時刻表が 1,000 件で切れる。各ホストの全件ダウンロード用の URL（`/api/v4/odpt:TrainTimetable.json`・`odpt:StationTimetable.json`）で取る
- チャレンジ限定ライセンスのデータ（JR東日本・京王・東武・京急・東急・西武・小田急）は、公共交通オープンデータチャレンジへの応募が前提で、2027年3月12日に許諾が終わる

### フロントエンド（`frontend/`）

- React 19 + Vite + Tailwind v4 + shadcn/ui（`components/ui`）、TanStack Query、React Router。パスエイリアス `@` は `src/` を指す
- API 呼び出しは `src/api.ts`、レスポンス型は `src/types.ts` に集約している。バックエンドの DTO を変えたらこの2ファイルも合わせる
- ODPT の ID を日本語ラベルに変換する処理は `src/lib/odpt.ts`
- ID は `odpt.Station:...` のように `:` や `.` を含むので、URL に入れるときは `encodeURIComponent` する
- `components/ui` は base-ui ベース。`PopoverTrigger` などのトリガーに `Button` を使うときは、子要素にせず `render={<Button ... />}` で渡す（子にすると button が入れ子になる）
- `components/ui` は shadcn の生成物で書式が違う（行末のセミコロンが無い）。Prettier は変えたファイルだけにかけ、`npx prettier --write src` のようにまとめてかけない
- 色は `index.css` のトークン（`primary`・`brand`・`warning`・`destructive-foreground` など）で指定し、色コードを直接書かない。路線の色・記号は `GET /api/routes`（ODPT の `odpt:color`・`odpt:lineCode`）を `lib/railways.ts` の `useRailways` / `useRailway` で引く

### インフラ（`infra/`）

CloudFront が `/api/*` を API Gateway（HTTP API）→ Lambda（`provided.al2023`, arm64, 1024MB。他社のデータを埋め込むため 512MB 以上が要り、起動を速くするため CPU の多い 1024MB にしている）に流し、それ以外を S3 に流す。リージョンは `ap-northeast-1`。

AI エージェントの API キー（Gemini）は SSM Parameter Store の SecureString（`/train-status-app/dev/gemini-api-key`）に手で登録する。Terraform・tfvars・Lambda の環境変数には置かない。キーの値がこの会話に出ないよう、ユーザーに `! aws ssm put-parameter --region ap-northeast-1 --type SecureString --overwrite --name /train-status-app/dev/gemini-api-key --value 'AIza...'` を実行してもらう（引用符の中はキーだけにする。以前、例の `<キー>` の `<` `>` まで登録されて Gemini が `API_KEY_INVALID` を返した）。利用上限は DynamoDB（`train-status-app-dev-ai-usage`）で数え、アプリ全体の1日の上限は `ai_calls_per_day`（tfvars）で変える。

ODPT のキーも同じく SSM の SecureString（`/train-status-app/dev/odpt-consumer-key`・`/train-status-app/dev/odpt-challenge-consumer-key`）に手で登録する。読む権限は `infra/bootstrap/lambda_odpt.tf`。

SPA のルーティングは CloudFront Function（`infra/main/functions/spa_rewrite.js`）で `/index.html` に書き換えている。`custom_error_response` は `/api/*` のエラーまで `index.html` の 200 にしてしまうので使わない。

CloudFront は標準の証明書（`cloudfront_default_certificate = true`）を使っている。この場合 AWS が `minimum_protocol_version` を `TLSv1` に固定するので、指定しない（指定すると `plan` に毎回差分が出る）。独自ドメインと ACM 証明書にしたら指定する。
