# ログイン・お気に入り・運行情報の通知 設計書

- 状態: 段階1（ログイン）を実装中（2026-10-18）
- 対象: Google アカウントでのログイン、お気に入り（駅・路線・経路）、運行情報の Web Push 通知
- 関連: [AI 鉄道エージェント 設計書](ai-api.md)、[経路探索エンジン 設計書](route-search.md)

---

## 1. 目的

Google アカウントでログインした利用者が、次のことをできるようにする。

1. **お気に入り**: よく使う駅・路線・経路（出発駅と到着駅の組）を登録し、ホームからすぐ開く。スマホと PC で同じものを使う
2. **運行情報の通知**: 登録した路線で、決めた曜日・時間帯に遅れや見合わせが起きたら、Web Push で知らせる（例: 平日 7:00〜9:30 の三田線）

---

## 2. 基本方針

1. **ログインしなくても今の機能はすべて使える。** ログインで増えるのはお気に入りと通知だけにする。ポートフォリオとして見る人にログインを求めない。
2. **個人情報をほとんど持たない。** 保存するのは Google の利用者 ID（ID トークンの `sub`）と、利用者が登録した内容だけにする。メールアドレス・名前・顔写真は保存しない。
3. **退会でデータをすべて消せる。** 利用者が自分で退会でき、そのとき利用者のデータをすべて消す。
4. **秘密の値は SSM に置く。** VAPID の秘密鍵は SSM Parameter Store の SecureString に手で登録する。Google の OAuth クライアント ID は秘密ではないので、tfvars と Lambda の環境変数、フロントのビルド時の環境変数に置いてよい。
5. **新しい依存は最小限にする。** Cognito は使わず、Google の ID トークンをバックエンドで確かめて、自前のセッションを発行する（3.1）。

---

## 3. ログイン

### 3.1 方式

Google Identity Services（「Google でログイン」ボタン）で ID トークンを受け取り、バックエンドで確かめてセッションを発行する。

```
ブラウザ ──「Google でログイン」──▶ Google
   │◀──────── ID トークン（JWT）──────┘
   │
   ├─ POST /api/auth/google { credential } ──▶ Lambda
   │                                              ├ ID トークンを確かめる（署名・aud・iss・exp）
   │                                              ├ 利用者がいなければ作る（sub で識別）
   │◀── Set-Cookie: __Host-session=<乱数> ────────┘ セッションを DynamoDB に保存
   │
   └─ 以後の /api/me/* は Cookie で利用者を特定する
```

Cognito を使わない理由:

- 必要なのは「誰か」を特定することだけで、ユーザー管理・パスワード・MFA は要らない
- Cognito の User Pool・ドメイン・IdP の Terraform と、トークンの更新の処理が増える
- ID トークン（RS256 の JWT）の検証は、標準ライブラリで書ける（`internal/auth/google.go`）。`google.golang.org/api/idtoken` は依存が大きく、実行ファイルが大きくなる（Lambda の起動が遅くなる）ので使わない。Google の公開鍵（`https://www.googleapis.com/oauth2/v3/certs`）は `Cache-Control` の `max-age` のあいだ使い回す

メールアドレス（パスワードやメールのコード）でのログインは作らない。パスワードのハッシュ・再設定・総当たり対策・メールの送信（SES は独自ドメインと本番利用の申請が要る）が増え、メールアドレスという個人情報も持つことになるため。必要になったら Cognito に移し、Google とメールの両方を Cognito で受け付ける（利用者は `sub` で付け替える）。

### 3.2 セッション

- セッション ID は 32 バイトの乱数。DynamoDB には SHA-256 のハッシュだけを保存する（テーブルが漏れても使えない）
- 有効期限は 30 日。DynamoDB の TTL で自動で消える。使われたら期限を延ばす（1日に1回まで書き込む）
- Cookie は `__Host-session`、`HttpOnly; Secure; SameSite=Lax; Path=/`。フロントの JS からは読めない
- 手元（`http://localhost`）では `Secure` が付けられないので、名前を `session` にして `Secure` を外す（`AWS_LAMBDA_RUNTIME_API` が無いとき）

### 3.3 CSRF 対策

フロントと API は CloudFront の同じオリジンなので、`SameSite=Lax` で他サイトからの POST には Cookie が付かない。加えて、Go 1.25 の `http.CrossOriginProtection` を router 全体にかけ、別のオリジンからの状態を変えるリクエスト（POST・PUT・DELETE など）を 403 にする。ブラウザの `Sec-Fetch-Site` を見る（無ければ `Origin` と `Host` を比べる）ので、許すオリジンの設定は要らない（CloudFront の後ろでは `Host` が API Gateway のものになり、`Origin` と比べられないため）。

### 3.4 API

| メソッド・パス | 内容 |
|---|---|
| `GET /api/auth/config` | Google の OAuth クライアント ID。ログインを使えない（設定が無い）ときは 503 で、画面はログインのボタンを出さない |
| `POST /api/auth/google` | ID トークンを受け取り、セッションを発行する |
| `POST /api/auth/logout` | セッションを消す |
| `GET /api/me` | ログイン中なら 200（利用者の作成日など）、していなければ 401 |
| `DELETE /api/me` | 退会する。利用者のデータ・セッション・通知の購読をすべて消す |

ログインが要る API は、ミドルウェアで Cookie からセッションを引き、`context` に利用者 ID を入れる。セッションが無ければ 401 を返す。

### 3.5 Google 側の準備（手作業）

- Google Cloud のプロジェクトで OAuth 同意画面を作る
- 「Google でログイン」のボタンが返す ID トークンには、スコープにかかわらずメールアドレス・名前・顔写真も入る。バックエンドは `sub` 以外を読まず、保存もしない
- OAuth クライアント ID（ウェブアプリケーション）を作り、承認済みの JavaScript 生成元に CloudFront のドメインと `http://localhost:5173` を登録する
- 同意画面にプライバシーポリシーの URL が要るので、`/privacy` のページを作る（7章）

---

## 4. お気に入り

### 4.1 登録できるもの

| 種類 | 中身 | 上限 |
|---|---|---|
| 駅 | 駅 ID | 30 |
| 路線 | 路線 ID | 30 |
| 経路 | 出発駅 ID・到着駅 ID | 20 |

上限は乱用対策のため。名前（駅名・路線名）は保存せず、表示のときに `GET /api/stations`・`GET /api/routes` から引く。データの更新で ID が無くなったものは、表示しないだけにして消さない。

### 4.2 API

| メソッド・パス | 内容 |
|---|---|
| `GET /api/me/favorites` | お気に入りの一覧（種類ごと） |
| `PUT /api/me/favorites/{kind}/{id}` | 登録する（経路の `id` は `<出発駅ID>~<到着駅ID>`） |
| `DELETE /api/me/favorites/{kind}/{id}` | 外す |

ID が assets に無いものは 400 を返す。

### 4.3 画面

- 駅・路線・経路検索の結果の画面に星のボタンを置く。ログインしていないときに押したら、ログインを勧める
- ホームの上部に「お気に入り」を出す。経路は、押すとその区間の経路検索を今の時刻で開く。路線は、今の運行情報を並べて出す
- ホームの運行情報の並び順（`lib/railwayOrder.ts`、`localStorage`）は今のまま端末ごとに持つ

---

## 5. 運行情報の通知

### 5.1 しくみ

```
EventBridge（5分ごと、5:00〜翌1:00）
   │
   ▼
通知用の Lambda（API と同じ実行ファイル、APP_MODE=notify）
   ├ 運行状況を取る（service の railwayConditions と同じ処理）
   ├ 路線ごとの状態を、前回の状態（DynamoDB）と比べる
   ├ 状態が変わった路線を見ている利用者を、通知の条件の索引（GSI）から引く
   ├ 曜日・時間帯が合う利用者の端末に Web Push を送る
   └ 今回の状態を保存する
```

- API の Lambda とは別の関数にする。API の同時実行やタイムアウトの設定と切り離すため。zip は同じものを使い、`cmd/api/main.go` が環境変数 `APP_MODE` で起動のしかたを変える
- ODPT への問い合わせは5分に1回（1日 240 回ほど）で、利用者の数には比例しない

### 5.2 路線の状態と通知する変化

路線の状態を次の3つに分ける。

| 状態 | 判定 |
|---|---|
| 平常 | 下の2つに当たらない |
| 遅れ | 列車の遅れの中央値（`medianDelays`）が 10 分以上の方向がある、または運行情報の文章が平常の文章でない（平常の判定は `service/assistant.go` の `normalWords` と同じ。平常時も「現在、１５分以上の遅延はありません。」のような文章が配信される） |
| 見合わせ | `isSuspended` が真（経路検索と同じ判定） |

- 状態が変わったときだけ通知する（平常→遅れ、遅れ→見合わせ、見合わせ→平常 など）。同じ状態が続く間は送らない
- 文章だけが変わったとき（「10分遅れ」→「20分遅れ」）は送らない
- 運行情報を取れなかった事業者（`statusOperators` に無い）の路線は、状態を変えない（取れないだけで「平常に戻った」と送らないため）
- 通知の本文は路線名と運行情報の文章だけにする。押すとその路線の画面を開く

### 5.3 通知の条件

利用者は路線ごとに、曜日（平日・土休日。祝日は `internal/calendar` で判定する）と時間帯（開始・終了。30分単位）を決める。上限は 10 路線。時間帯の外で起きた変化は、あとから送らない。

### 5.4 Web Push

- VAPID の鍵の組を作り、秘密鍵は SSM（`/train-status-app/dev/vapid-private-key`）に手で登録する。公開鍵はフロントのビルド時の環境変数に置く
- 送信は `github.com/SherClockHolmes/webpush-go` を使う
- フロントに Service Worker（`public/sw.js`）を足し、`push` で通知を出し、`notificationclick` で画面を開く
- 購読（endpoint と鍵）は端末ごとに保存する。1人 5 端末まで。送ったときに 404・410 が返った購読は消す
- iPhone は、ホーム画面に追加した Web アプリ（iOS 16.4 以降）でしか Web Push を受け取れない。Web App Manifest を足し、通知の設定の画面で案内する

### 5.5 API

| メソッド・パス | 内容 |
|---|---|
| `GET /api/me/alerts` | 通知の条件の一覧 |
| `PUT /api/me/alerts/{railwayId}` | 条件を登録・変更する（曜日・時間帯） |
| `DELETE /api/me/alerts/{railwayId}` | 条件を消す |
| `POST /api/me/push-subscriptions` | この端末の購読を登録する |
| `DELETE /api/me/push-subscriptions` | この端末の購読を消す（本文で endpoint を渡す） |
| `POST /api/me/push-subscriptions/test` | この利用者の端末に試しの通知を送る（1分に1回まで） |

---

## 6. データ

DynamoDB に1つのテーブル（`train-status-app-dev-users`、オンデマンド）を作る。

| pk | sk | 中身 |
|---|---|---|
| `USER#<sub>` | `PROFILE` | 作成日 |
| `USER#<sub>` | `SESSION#<セッションIDの SHA-256>` | `expiresAt`（TTL）。退会のときにセッションを引くための目印 |
| `USER#<sub>` | `FAV#STATION#<駅ID>` など | 登録日 |
| `USER#<sub>` | `ALERT#<路線ID>` | 曜日・時間帯。GSI 用に `gsi1pk = ALERT#<路線ID>` |
| `USER#<sub>` | `PUSH#<endpoint の SHA-256>` | endpoint・鍵 |
| `SESSION#<セッションIDの SHA-256>` | `SESSION` | 利用者 ID、`expiresAt`（TTL） |
| `NOTIFY` | `STATE` | 通知用の前回の状態（全路線を1つの項目にまとめ、1回の確認で読み書き1回ずつにする） |

- GSI `gsi1`（`gsi1pk`）で、路線を見ている利用者を引く
- セッションは `SESSION#<hash>` のほかに、利用者の側にも目印（`USER#<sub>` / `SESSION#<hash>`、TTL 付き）を書く
- 退会は `pk = USER#<sub>` を Query して全部消し、目印から引いたセッションも消す
- 手元では、テーブル名の環境変数が無ければメモリ上の実装を使う（`ai/fake` と同じ考え方）。service のテストもメモリ上の実装で行う

---

## 7. プライバシーポリシー

`/privacy` のページを足し、フッターからリンクする。ログイン・ログアウト・退会は `/account` の画面で行い、ヘッダーの右端（スマホでも出す）からリンクする。書く内容:

- 保存するもの（Google の利用者 ID、お気に入り、通知の条件、通知の購読）と、受け取っても保存しないもの（メールアドレス・名前・顔写真）
- 使い道（お気に入りと通知のためだけ）
- 退会すると全部消えること
- Cookie はログインの状態を保つためだけに使うこと

---

## 8. インフラ

| 場所 | 足すもの |
|---|---|
| `infra/main` | users テーブル（GSI・TTL）、通知用の Lambda、EventBridge のルールと `aws_lambda_permission` |
| `infra/bootstrap` | Lambda の実行ロールに付けるポリシー（users テーブルの読み書き、VAPID の SSM の読み取り）。deploy 用ロールに EventBridge のルールを扱う権限 |
| tfvars | `google_client_id` |

- EventBridge のルールから Lambda を呼ぶのは、リソースベースのポリシー（`aws_lambda_permission`）で済むので、新しい IAM ロールは要らない（EventBridge Scheduler だと IAM ロールが要る）
- CloudFront の `/api/*` はキャッシュ無効で、オリジンリクエストポリシーが `AllViewerExceptHostHeader` なので、Cookie はそのまま Lambda に届く。変更は要らない

---

## 9. 段階的な導入

| 段階 | 内容 |
|---|---|
| **1** | ログイン・ログアウト・退会、プライバシーポリシーのページ、users テーブル |
| **2** | お気に入り（API・星のボタン・ホームの表示） |
| **3** | 通知（Service Worker・購読・条件の設定画面・通知用の Lambda・EventBridge） |

段階ごとに PR を分ける。1 と 2 は合わせてもよい。

---

## 10. テスト

- ID トークンの検証は、検証の部分をインターフェースにして、テストでは偽物を使う
- セッション（発行・期限・ログアウト）、CSRF の `Origin` の確認、ログインが要る API の 401 を handler のテストで確かめる
- 通知の判定（状態の変化・時間帯・曜日・取れなかった事業者）は、時刻と運行状況を与える純粋な関数にしてテストする
- Web Push の送信はインターフェースにして、テストでは送った内容を記録する偽物を使う

---

## 11. 未決事項

- `*.cloudfront.net` を Google の承認済みの JavaScript 生成元に登録できるか。できなければ独自ドメインと ACM 証明書が要る
- 遅れとみなす中央値のしきい値（仮に 10 分）。実際の通知の頻度を見て決める
- 通知を確かめる間隔（仮に 5 分）と、夜間に止める時間帯
- ODPT チャレンジ限定ライセンスのデータ（2027年3月12日に許諾が終わる）を通知に使う路線の、許諾が終わったあとの扱い
