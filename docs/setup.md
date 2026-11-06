# 構築・移行の手順

このアプリを新しい AWS アカウントや別のリポジトリに作るときに、入れる値と実行する順番をまとめる。
今の環境（`train-status-app-dev`、`ap-northeast-1`）をそのまま使う間は、この手順は要らない。

各段階の詳しい説明は `infra/CLAUDE.md`、他社のデータは `backend/CLAUDE.md` と `docs/design/multi-operator.md` にある。

## 0. 前もって用意するもの

| もの | 入手先 | 使う場所 |
|---|---|---|
| AWS アカウントと、手元の管理者の認証情報 | | bootstrap の適用・SSM への登録 |
| ODPT の開発者のキー | [ODPT の開発者サイト](https://developer.odpt.org/) | 都営以外の事業者の運行情報・列車位置・データの取得 |
| ODPT のチャレンジのキー | 公共交通オープンデータチャレンジ（許諾は 2027-03-12 まで） | JR東日本・京王・東武・京急・東急・西武・小田急 |
| Gemini の API キー | Google AI Studio | AI チャット。無いと `/api/chat` が 503 を返す |
| Google の OAuth クライアント ID | Google Cloud Console（ウェブ アプリケーション） | ログイン。無いとログインが使えない |

キーの値は、画面・会話・ログ・tfvars・Terraform に出さない（ルートの `CLAUDE.md` の「守ること」）。

## 1. 名前とアカウント ID を書き換える

S3 のバケット名は世界で一意なので、別のアカウントに作るなら `project_name` か `env` を変える。
リソース名・SSM のパラメータ名は `<project_name>-<env>`・`/<project_name>/<env>/...` から作られる。
バケット名は tfvars・`Makefile` にも書いてあるが、bootstrap と deploy 用ロールの権限は `<project_name>-<env>` から名前を作るので、
`frontend_bucket_name` は `<project_name>-<env>-frontend-assets`、`lambda_artifact_bucket_name` は `<project_name>-<env>-lambda-artifacts` にそろえる。

| 場所 | 書き換えるもの |
|---|---|
| `infra/env/dev.tfvars` | `aws_region`・`project_name`・`env`・`frontend_bucket_name`・`lambda_artifact_bucket_name`・`tags`（bootstrap と main の両方が読む） |
| `infra/env/dev.tfvars` | `google_client_id`（秘密の値ではない）・`odpt_operators`・`ai_calls_per_day`（Gemini の無料枠の 80%） |
| `infra/bootstrap/variables.tf` | `github_repository`（OIDC で信頼するリポジトリ `owner/name`） |
| `infra/main/provider.tf` | `backend "s3"` の `bucket`・`dynamodb_table`・`region`（bootstrap が作る tfstate のバケットとロックのテーブル） |
| `Makefile` | `FRONTEND_BUCKET`・`LAMBDA_ARTIFACT_BUCKET` |
| `.github/workflows/ci.yml`（`AWS_PLAN_ROLE_ARN`）・`deploy.yml`（`AWS_DEPLOY_ROLE_ARN`） | AWS のアカウント ID とロール名。bootstrap の出力（`github_plan_role_arn`・`github_deploy_role_arn`）と同じにする |
| `backend/cmd/api/main.go` の `pushSubject` | Web Push のサービスが連絡に使う URL（リポジトリの URL か `mailto:`） |
| `backend/assets/extra.version` | 4 で作り直す |

## 2. bootstrap を適用する

```bash
make tf-bootstrap-init
make tf-bootstrap-plan
make tf-bootstrap-apply
```

tfstate・Lambda のアーティファクト用の S3、ロックのテーブル、GitHub Actions の OIDC プロバイダとロール、Lambda の実行ロール、通知の SNS トピック、予算を作る。

- bootstrap の state は手元（`infra/bootstrap/terraform.tfstate`、gitignore 済み）にしか無い。手元の環境を移すときは、このファイルも移す
- GitHub の OIDC プロバイダはアカウントに1つしか作れない。アカウントにすでにあるなら、`terraform import aws_iam_openid_connect_provider.github <ARN>` で取り込む
- 予算はアカウント全体の費用を見る。無料の範囲は予算2つ・標準のアラーム10個まで（アカウント全体の数）

## 3. 秘密の値を SSM に登録する

ユーザーが手で実行する（`!` を付けると Claude Code の会話の中で実行できる）。`<project>/<env>` は 1 で決めた名前にする。

```bash
! aws ssm put-parameter --region ap-northeast-1 --type SecureString --name /<project>/<env>/gemini-api-key --value 'キー'
! aws ssm put-parameter --region ap-northeast-1 --type SecureString --name /<project>/<env>/odpt-consumer-key --value 'キー'
! aws ssm put-parameter --region ap-northeast-1 --type SecureString --name /<project>/<env>/odpt-challenge-consumer-key --value 'キー'
! cd backend && aws ssm put-parameter --region ap-northeast-1 --type SecureString --name /<project>/<env>/vapid-private-key --value "$(go run ./cmd/vapid-key)"
```

- 引用符の中はキーだけにする（`<` `>` まで登録すると、Gemini が `API_KEY_INVALID` を返す）
- VAPID の鍵は、鍵を作ってそのまま登録する（値を画面に出さない）。作り直すと利用者の今の購読がすべて切れるので、登録したあとは作り直さない（`--overwrite` を付けない）。移行で鍵を引き継がないなら、利用者は通知を設定し直す
- 登録できたかは、名前だけで確かめる: `aws ssm describe-parameters --region ap-northeast-1 --query 'Parameters[].Name'`

## 4. 他社のデータを S3 に置く

都営以外の事業者のデータは、ライセンス（ODPT の第8条4項）のためリポジトリに入っていない。`make backend-build` は `backend/assets/extra/` がそろっていないと失敗する。

```bash
# backend/.env に ODPT_CONSUMER_KEY・ODPT_CHALLENGE_CONSUMER_KEY を書いてから
cd backend && sh scripts/update_assets.sh --extra-only && cd ..
make backend-extra-upload   # S3 の assets-extra/<版>/ に置き、assets/extra.version を書き換える
```

`backend/assets/extra.version` はコミットする（版の名前だけ）。`assets/extra/` はコミットしない。

## 5. バックエンド・インフラ・フロントを反映する

順番に1つずつ実行し、前のものが成功してから次に進む。

```bash
make backend-deploy    # Lambda の zip を S3 に置く（main の data "aws_s3_object" が読むので、main より先）
make tf-main-init
make tf-main-plan
make tf-main-apply     # CloudFront・API Gateway・Lambda・DynamoDB・アラームなど
make frontend-deploy
```

`cd infra/main && terraform output -raw cloudfront_domain_name` がアプリの URL になる。

## 6. AWS の外の設定

- **Google Cloud Console**: OAuth クライアントの「承認済みの JavaScript 生成元」に `https://<CloudFront のドメイン>` を足す（手元で試すなら `http://localhost:5173` も）。足さないとログインできない
- **GitHub の Ruleset**: main を保護し、必須チェックを `changes`・`backend`・`frontend`・`infra` にする（`infra/CLAUDE.md` の「Ruleset」）
- **GitHub のシークレット・変数**: 要らない（CI は OIDC で AWS のロールを引き受ける）
- **監視のメール**（任意）: 通知を受けたいなら、SNS トピックにメールを購読させる（`infra/CLAUDE.md` の「監視」）。購読しなければ、アラームと予算の状態は AWS のコンソールでだけ見られる

## 7. 手元で動かすときの `backend/.env`

| 変数 | 無いとき |
|---|---|
| `ODPT_CONSUMER_KEY`・`ODPT_CHALLENGE_CONSUMER_KEY` | 都営だけで動く |
| `GEMINI_API_KEY` | `/api/chat` が 503 を返す |
| `GOOGLE_CLIENT_ID` | ログインが使えない |
| `VAPID_PRIVATE_KEY` | 通知が使えない |

変数名だけを見るときは `sed 's/=.*/=<hidden>/' backend/.env`。

## 8. 確かめる

- `curl -s -o /dev/null -w '%{http_code}' https://<CloudFront のドメイン>/api/status` が 200
- 画面で、運行情報・経路検索・AI チャット・ログイン・通知の設定ができる
- `aws cloudwatch describe-alarms --region ap-northeast-1 --query 'MetricAlarms[].AlarmName'` にアラームが5個ある
- main へのマージで CI と `deploy` が成功する
