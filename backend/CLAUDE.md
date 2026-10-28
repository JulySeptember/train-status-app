# backend/CLAUDE.md

Go の REST API（Go 1.25, 標準 `net/http`）。ルートの CLAUDE.md の「守ること」も合わせて守る。

## コマンド

### Swagger

`backend/docs/` は swaggo が生成したファイル（手で編集しない）。handler のアノテーションや DTO を変えたら再生成する:

```bash
cd backend && swag init -g cmd/api/main.go -o docs
```

手元の `swag` が go.mod の swaggo/swag と違うバージョンだと CI で差分が出る。`go run github.com/swaggo/swag/cmd/swag@<go.mod のバージョン> init -g cmd/api/main.go -o docs` で実行する。

### AI エージェントの評価セット

評価セット（`internal/ai/testdata/eval.yaml`）は実際の Gemini を呼ぶので、手で実行する（CI では動かない。無料枠を数十回使い、約7分かかる）。キーは SSM から直接渡し、画面や会話に出さない:

```bash
cd backend && AI_EVAL=1 GEMINI_API_KEY="$(aws ssm get-parameter --region ap-northeast-1 --name /train-status-app/dev/gemini-api-key --with-decryption --query Parameter.Value --output text)" go test ./internal/ai -run 'TestEval$' -v -timeout 30m
```

### 静的データの更新

`scripts/update_assets.sh` が ODPT から `assets/*.json` を取り直す。`station_timetable.json` / `train_timetable.json` を更新したら、埋め込み用の軽量データを再生成する（忘れると `assets` のテストが失敗する）:

```bash
make backend-generate   # = cd backend && go generate ./assets
```

同じスクリプトが、`backend/.env` のキーがあれば都営以外の事業者のデータも取り、都内に絞って `assets/extra/`（gitignore 済み。コミットしない）に置く。他社のデータだけを取り直すときは `sh scripts/update_assets.sh --extra-only`。取得した元の JSON（約370MB）と区市町村の境界は `backend/.odpt-cache/` に残る。駅の中の案内（`docs/design/station-guide.md`）も同じスクリプトで、`assets/guide/<駅>.yaml`（手で作る対応表。コミットする）のある駅の JR の歩行空間ネットワーク・構内地図を `.odpt-cache/station-guide/` に取り、`assets/extra/station_guide.json` を作る。対応表を直したときは `go run ./cmd/gen-assets -kind guide -raw .odpt-cache -labels assets/guide -out assets/extra/station_guide.json -preview .odpt-cache/preview` で作り直し、`-preview` の SVG（ホーム・前の端・改札の位置）で確かめる（SVG もデータを描いたものなので、gitignore した場所に書く）。`extra/` はディレクトリごと埋め込むので、`station_guide.json` のように版（`EXTRA_FILES`）に入っていないファイルがあると `make backend-build` が失敗する。都内の駅の一覧 `assets/tokyo_stations.txt`（駅IDだけなのでコミットする）もこのとき作り直す。`extra/` のデータは `assets.New(assets.WithExtra())` で読み込み、無ければ都営だけで動く（CI のテスト）。`extra/` のデータは `go:embed` で実行ファイルに埋め込まれるので、データを差し替えて結果を比べるときは、差し替えるたびにビルドし直す（`go run` もビルドし直す。前にビルドした実行ファイルは古いデータのまま）。

本番に入れる `extra/` は、非公開の S3（Lambda アーティファクト用のバケットの `assets-extra/<版>/`）に置き、使う版を `assets/extra.version`（版の名前だけなのでコミットする）に書く。他社のデータを取り直したら `make backend-extra-upload`（S3 へのアップロード。実行前にユーザーへ確認する）で版を作り、`extra.version` をコミットする。`make backend-build` は `extra/` がそろっていなければ失敗し、`make backend-deploy` は先に `make backend-extra-download` で `extra.version` の版を取ってくる（手元の `extra/` は上書きされる）。取り直しの頻度と手順は `docs/design/multi-operator.md` 4章。

## アーキテクチャ

- `cmd/api/main.go`: `AWS_LAMBDA_RUNTIME_API` があれば `aws-lambda-go-api-proxy` の httpadapter（API Gateway HTTP API, payload v2）で動き、なければ通常の HTTP サーバとして動く。CORS はローカル実行時だけ有効
- 層構成は `router` → `handler` → `service` → `client`（ODPT へのリアルタイム取得）/ `assets`（静的データ）
  - `handler` は service のセンチネルエラー（`ErrStationNotFound`、`ErrTrainNotFound`、`ErrExternalAPI` など）を HTTP ステータスに変換する
  - `service` はデータの加工・集約を担い、`model`（ODPT JSON-LD そのままの型）をフロント向けの DTO に変換する。DTO は service.go に定義する
  - `client` で外部 API を呼ぶのは運行情報（`odpt:TrainInformation`）と列車位置（`odpt:Train`）だけ。共通処理はジェネリクスの `fetch[T]`。事業者ごと（`client.Sources`）に並列に取り、一部が失敗しても残りを返す（全部失敗したときだけ `ErrExternalAPI`）。都営以外の事業者は環境変数 `ODPT_OPERATORS`（tfvars の `odpt_operators`）に書いたものだけを取る。キーは手元では `ODPT_CONSUMER_KEY`（`api.odpt.org`）・`ODPT_CHALLENGE_CONSUMER_KEY`（`api-challenge.odpt.org`）（`backend/.env`）、Lambda では SSM から読む（`internal/odptkey`）
- `assets/`: ODPT の静的データ（路線・駅・駅時刻表・列車時刻表・列車種別）を `go:embed` で埋め込み、起動時に全件読み込む。駅時刻表と列車時刻表は、それぞれ約25MBの JSON を埋め込まず、使う項目に絞って文字列表にまとめた `station_timetable.gob` / `train_timetable.gob`（`assets/slim`、`cmd/gen-assets` で生成）を埋め込む。Lambda のコールドスタートを短くするため。`model.StationTimetableEntry` も約64万件（都営 約12万件・他社 約52万件）が常駐するので、使う項目以外を足さない。列車時刻表は経路探索用で、`model` の型に戻さず、文字列表の番号と分（3時前は +24時間）のまま `slim.TrainTimetables` で持つ
- `internal/route`: 経路探索エンジン（RAPTOR）。`GET /api/journeys` から使う。遅れは `Query.Delays`（路線・方向ごと）で受け取り、パターンの時刻に足して探す（`shift`）。service（`service/realtime.go`）が `odpt:Train` の遅れの中央値と、運行情報の文章による見合わせの判定を30秒キャッシュして渡す。`slim.TrainTimetables` を数値のまま使い、駅・路線は ID の文字列で扱う（都営に決め打ちしない）。乗り換えの対応表（`route.Transfer`）は service（`service/transfer.go`）が作って渡す。乗り換えの決め方・同じ名前の駅のまとめ方は `docs/design/route-search.md`
- `internal/ai`: AI エージェント（`POST /api/chat`）。AI が道具（`tools.go`）を呼び、アプリが service の照会（`service/assistant.go`）を実行して結果を返す、を最大5往復繰り返す。Provider は Gemini（`ai/gemini`、REST を直接呼ぶ）とテスト用の `ai/fake`。駅名の特定は `internal/station`。API キーが無いとき（手元で `GEMINI_API_KEY` を設定していないとき、Lambda では SSM の準備ができるまで）は `/api/chat` が 503 を返す。設計は `docs/design/ai-api.md`
- `internal/calendar`: 運行日（3時前は前日扱い）と、その日に適用されるダイヤ種別（`odpt.Calendar:*`）を判定する。祝日は祝日法に基づいて計算する
- `internal/auth`: Google アカウントでのログイン（`/api/auth/*`・`/api/me`）。`internal/favorite` はお気に入り（`/api/me/favorites`）。どちらも DynamoDB の1つのテーブル（`train-status-app-dev-users`、`auth/dynamostore`）に置き、利用者の項目は pk `USER#<sub>`。退会と同時の書き込みで項目が残らないよう、セッション・お気に入りは PROFILE があることを条件に TransactWriteItems で書く（新しい種類の項目を足すときも同じにする）。`USERS_TABLE` が無い手元ではメモリ上（`auth.MemoryStore`、再起動で消える）。保存するのは Google の `sub` だけ。設計は `docs/design/login.md`
- service のテストは `mockClient` と実際の embed アセットを使う。route のテストは小さな架空の路線網で確かめる

## ODPT データの注意点

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
- 大江戸線の環状部を回って光が丘方面へ続ける列車は、都庁前を同じ列車ID で2回発車する（駅時刻表にも2回載る）。駅・列車ID の組で時刻を1つに決めない
- ODPT の API は1回の取得で最大1,000件しか返さない。列車時刻表・駅時刻表は、各ホストの全件ダウンロード用の URL（`/api/v4/odpt:TrainTimetable.json`・`odpt:StationTimetable.json`、リダイレクトされる）から取る
- ODPT のデータの一覧は `https://ckan.odpt.org/dataset/?_organization_limit=0&tags=%E9%89%84%E9%81%93-railway`（鉄道）。CKAN の API は使えないので HTML から読む。データセットのページは、ログインしないとファイルのリンクが出ない。チャレンジのファイルは `https://api-challenge.odpt.org/api/v4/files/<事業者>/data/<ファイル名>?acl:consumerKey=<チャレンジのキー>` で取れる（一時的な署名付きの URL にリダイレクトされる）。題名に【公開終了】と付いたデータセットは取れない。都営は JSON のほかに GTFS / GTFS-RT もあるが使っていない（理由は `docs/design/route-search.md` 3章）
- 都営以外の事業者のデータは、キーの要る `api.odpt.org`（公共交通オープンデータ基本ライセンス）と `api-challenge.odpt.org`（チャレンジ限定ライセンス）から取る。今の `backend/assets` は都営（CC BY 4.0）だけなので置いてよい。他社のデータの置き場所は `docs/design/multi-operator.md` 4章
- チャレンジ限定ライセンスのデータ（JR東日本・京王・東武・京急・東急・西武・小田急）は、公共交通オープンデータチャレンジへの応募が前提で、2027年3月12日に許諾が終わる
