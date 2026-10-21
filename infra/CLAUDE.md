# infra/CLAUDE.md

Terraform で管理する AWS の構成と、デプロイ・CI の詳細。ルートの CLAUDE.md の「守ること」も合わせて守る（AWS に反映される操作は実行前にユーザーへ確認する）。

## 構成

CloudFront が `/api/*` を API Gateway（HTTP API）→ Lambda（`provided.al2023`, arm64, 1024MB。他社のデータを埋め込むため 512MB 以上が要り、起動を速くするため CPU の多い 1024MB にしている）に流し、それ以外を S3 に流す。リージョンは `ap-northeast-1`。

SPA のルーティングは CloudFront Function（`main/functions/spa_rewrite.js`）で `/index.html` に書き換えている。`custom_error_response` は `/api/*` のエラーまで `index.html` の 200 にしてしまうので使わない。

CloudFront は標準の証明書（`cloudfront_default_certificate = true`）を使っている。この場合 AWS が `minimum_protocol_version` を `TLSv1` に固定するので、指定しない（指定すると `plan` に毎回差分が出る）。独自ドメインと ACM 証明書にしたら指定する。

## API キー（SSM）

AI エージェントの API キー（Gemini）は SSM Parameter Store の SecureString（`/train-status-app/dev/gemini-api-key`）に手で登録する。Terraform・tfvars・Lambda の環境変数には置かない。キーの値が会話に出ないよう、ユーザーに `! aws ssm put-parameter --region ap-northeast-1 --type SecureString --overwrite --name /train-status-app/dev/gemini-api-key --value 'AIza...'` を実行してもらう（引用符の中はキーだけにする。以前、例の `<キー>` の `<` `>` まで登録されて Gemini が `API_KEY_INVALID` を返した）。利用上限は DynamoDB（`train-status-app-dev-ai-usage`）で数え、アプリ全体の1日の上限は `ai_calls_per_day`（tfvars）で変える。

ODPT のキーも同じく SSM の SecureString（`/train-status-app/dev/odpt-consumer-key`・`/train-status-app/dev/odpt-challenge-consumer-key`）に手で登録する。読む権限は `bootstrap/lambda_odpt.tf`。

## デプロイ

main へのマージで CI（`.github/workflows/ci.yml`）が、検証が通ったあとに `deploy.yml` を呼んで自動でデプロイする。変更のあった領域だけを backend → `terraform apply` → frontend の順に反映する。全領域をやり直すときは Actions から CI を main で手動実行する（`workflow_dispatch`）。

main への push が続くと、待機中の実行は新しいものに置き換わり、その push の変更が反映されないことがある（変更の判定は push 単位のため）。その場合も手動実行で反映する。

`deploy.yml` のジョブに GitHub Environment（`environment:`）を付けると、OIDC の `sub` が `repo:<owner>/<repo>:environment:<名前>` に変わり、deploy 用ロールを引き受けられなくなる。付けるときは `bootstrap/github_oidc.tf` の条件も変える。

手元の make（CD が失敗したときや bootstrap の適用に使う）:

- `make backend-deploy`: linux/arm64 でビルドして zip にし、S3 にアップロードする。Lambda への反映は `make tf-main-apply`（`s3_object_version` を参照している）
- `make frontend-deploy`: ビルドして S3 に sync し、CloudFront を invalidate する
- `make tf-main-plan` / `tf-main-apply`: `main` を `env/dev.tfvars` で適用する。`tf-*-apply` / `destroy` は `-auto-approve` 付き
- `bootstrap`: tfstate 用 S3・DynamoDB、Lambda アーティファクト用 S3、GitHub Actions 用の OIDC プロバイダと IAM ロール（PR の plan 用・main のデプロイ用）、Lambda の実行ロール（`lambda_role.tf`）とそれに付けるポリシー（`lambda_ai.tf`・`lambda_odpt.tf`）を作る。state はローカルにあるので、手元から `make tf-bootstrap-apply` で適用する。bootstrap を変えた PR は、マージ（CD の `terraform apply`）の前に bootstrap を適用する
- Lambda に権限を足すときは、`bootstrap` にポリシーを作って `aws_iam_role_policy_attachment.lambda` に足す。deploy 用ロールには、Lambda の実行ロールの `iam:PassRole` 以外の IAM の権限を与えない（信頼ポリシーやポリシーを変えられると、SSM の API キーを読めるロールを引き受けたり、権限を広げたりできる）

## Ruleset

main は Ruleset（`main`）で保護している: PR 必須（承認は不要）、force push・削除の禁止、必須チェックは `changes`・`backend`・`frontend`・`infra`（ci.yml のジョブ名）。`if` でスキップされたジョブは成功として扱われるので、変更のない領域があってもマージできる。`changes` が失敗すると後続がスキップされて通ってしまうため、`changes` も必須にしている。ci.yml のジョブ名を変えたり検証のジョブを足したりしたら、Ruleset の必須チェックも合わせて変える（`gh api repos/JulySeptember/train-status-app/rulesets`）。`infra-plan` は AWS 側の一時的な失敗やフォークからの PR があるので必須にしていない。

Swagger のチェックは go.mod の swaggo/swag と同じバージョンの CLI で再生成して差分を見る。

## 本番 Lambda の調査（読み取りのみ）

```bash
aws logs filter-log-events --region ap-northeast-1 \
  --log-group-name /aws/lambda/train-status-app-dev-api \
  --filter-pattern '"REPORT"'   # Init Duration / Max Memory Used を確認
```
