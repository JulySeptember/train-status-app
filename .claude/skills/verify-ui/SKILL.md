---
name: verify-ui
description: フロントエンド（またはフロントに出る API）を変えたあとに、手元でアプリを起動し、Playwright MCP で変更のあった画面を開いて、文言・操作・コンソールエラー・PC とスマホの幅の表示を確かめて報告する。「画面を確認して」「動作確認して」と言われたときや、frontend/ や DTO を変えた作業の終わりに使う。
---

# 画面の確認（verify-ui）

変更した画面を実際のブラウザで開き、決まった手順で確かめる。目的は、毎回同じ観点で確認し、結果を同じ形で報告すること。コードは直さず、見つけた問題を報告する（直すのはメインの会話）。

## 1. 確認する画面を決める

`git diff main...HEAD --stat`（コミット済み）と `git status`・`git diff --stat`（まだコミットしていないもの）で変わったファイルを見て、下の表から確認する画面を選ぶ。どの画面か決められないとき（`api.ts`・`types.ts`・`Layout.tsx`・`index.css`・`components/ui` など共通の部分の変更）は、表の画面をすべて確かめる。

| 画面 | URL | 主なファイル | 確かめること |
|---|---|---|---|
| 運行情報（トップ） | `/` | `pages/Home.tsx`、`RealtimeNotice.tsx` | 上に AI 乗換相談・経路検索のタブ、下に路線の一覧と運行状況が出る |
| 路線 | `/routes/odpt.Railway%3AToei.Asakusa` | `pages/Route.tsx`、`DirectionTabs.tsx` | 駅の並び。方向のタブで、駅へのリンクの `direction` が変わる（駅の並びは変わらない） |
| 駅の一覧 | `/stations` | `pages/Stations.tsx`、`StationSelect.tsx` | 駅が出て、絞り込める |
| 駅（時刻表） | `/stations/odpt.Station%3AToei.Oedo.Shinjuku` | `pages/Station.tsx`、`Timetable.tsx` | 平日・土休日の切り替え、次発の表示 |
| 列車 | 駅の時刻表の列車を押して開く | `pages/Train.tsx`、`TrainLocation.tsx` | 列車の位置（走っていなければ「まだ出発していません」などと始発駅・時刻） |
| 経路検索 | `/journeys?from=odpt.Station%3AToei.Asakusa.Asakusa&to=odpt.Station%3AToei.Oedo.Shinjuku` | `pages/Journey.tsx`、`JourneyForm.tsx`、`JourneyList.tsx` | 経路の一覧と乗り換え。遅れを反映しない検索は URL に `&realtime=false` を足して開く（フォームのチェックは「検索」を押すまで効かない） |
| AI チャット | `/chat` | `pages/Chat.tsx` | 画面が出ることだけ（下の注意を参照） |
| ライセンス | `/license` | `pages/License.tsx` | 出典の文言 |
| 存在しない URL | `/no-such-page` | `pages/NotFound.tsx` | 404 の画面 |

列車は、駅の画面の時刻表で「本日」と付いたダイヤの列車（リンクになっている）を押して開く。ほかのダイヤの列車はリンクにならない。
URL に入れる ID の `:` は `%3A` にする（アプリと同じく `encodeURIComponent` した形）。

## 2. 起動する

1. すでに動いているか確かめる: `curl -sf localhost:8080/health` と `curl -sf -o /dev/null localhost:5173`
2. 動いていなければ、Bash の `run_in_background` で起動する（`&` は付けない）:
   - バックエンド: `make backend-run`
   - フロントエンド: `make frontend-dev`
   - docker で動かしたいとユーザーに言われたときだけ `make up`
3. 上の `curl` が通るまで待つ。Bash の `run_in_background` で `until curl -sf localhost:8080/health >/dev/null && curl -sf -o /dev/null localhost:5173; do sleep 1; done` を動かし、終わったという通知を待つ
4. 自分で起動したサーバーは、最後に `TaskStop` で止め、上の `curl` が通らなくなったことを確かめる。もともと動いていたものは止めない

注意:

- WSL では、続けて編集したファイルを vite が読み落とし、古い画面が出ることがある。変更が画面に出ていないときは、まず vite を起動し直す
- `backend/assets/extra/` があれば他社の駅・路線も出る。無ければ都営だけになる。どちらで確かめたかを報告に書く
- `backend/.env` の中身は表示しない（ODPT のキーが入っている）

## 3. 各画面を確かめる

画面ごとに、次の順で行う。

1. `browser_resize` で幅 1280 × 高さ 800 にし、`browser_navigate` で開く
2. `browser_wait_for` の `textGone: "読み込み中..."` で、読み込みが終わるまで待つ（経路検索は数秒かかる。秒数を決めて待たない）
3. `browser_snapshot` で文言を読み、上の表の「確かめること」と、今回の変更で変わるはずの文言・要素を確かめる
4. 変更に関わる操作（タブの切り替え、絞り込み、検索、リンク）を `browser_click` / `browser_type` で行い、もう一度 `browser_snapshot` で結果を確かめる
5. `browser_console_messages` でエラーが出ていないか見る。`browser_network_requests` で `/api/*` が 4xx・5xx を返していないか見る
6. `browser_take_screenshot` で保存する。`filename` は `.playwright-mcp/verify-<画面>-pc.png`
7. `browser_resize` で幅 390 × 高さ 844 にし、もう一度 `browser_snapshot` とスクリーンショット（`.playwright-mcp/verify-<画面>-sp.png`）。撮った画像は Read で開いて見る。スマホの幅では下のナビ（`BottomNav`）が出て、ヘッダーのナビが隠れる。横にはみ出していないかも見る

スクリーンショットは `.playwright-mcp/`（gitignore 済み）にしか保存できない。`filename` はリポジトリのルートからのパスなので、`.playwright-mcp/` を付けて渡す。

駅の画面は時刻表が長く、`browser_snapshot` の結果が大きすぎて返ってこない。`target: "main"` と `filename: ".playwright-mcp/verify-<画面>.yml"` を付けてファイルに保存し、`grep` で見たい部分（`heading`・`tab`・`本日`・`次発` など）だけを読む。

### データについての注意

- 路線の記号（`lineCode`）が ODPT に無い路線は `lib/railways.ts` の `LINE_CODES` で補っている。記号の決まっていない八高線・相鉄直通線は、色だけの四角になる。今の仕様なので、変更と関係なければ問題として報告しない

- 運行情報・列車の位置・遅れは ODPT のリアルタイムのデータなので、時刻によって変わる。「遅れがある」「見合わせ中」などの表示を確かめたいときに、たまたまその状態の列車や路線が無ければ、無かったと報告する（データを書き換えて作らない）
- 深夜（終電後から始発まで）は列車の位置が無く、時刻表の次発が翌日になる。その時刻に確かめたことを報告に書く
- `/chat` は質問を送ると実際の Gemini を呼び、無料枠を使う。ユーザーに頼まれたときだけ送る。`GEMINI_API_KEY` が無ければ 503 になるのが正しい動き

### ログインしたあとの画面

Playwright のブラウザでは本物の Google でログインできない。ログインしたあとの表示（星のボタン、ホームのお気に入りなど）は、`browser_run_code_unsafe` で `page.route` を使い、`/api/me`（`{"createdAt": ...}`）や `/api/me/favorites` の応答を差し替えて確かめる。差し替えたことと、本物のログインでは確かめていないことを報告に書く。本物のログインでの確認は、手元でサーバーを起動したままにして、ユーザーに普段のブラウザで試してもらい、バックエンドのログ（ステータスとエラーの種類だけが出る）を見る。バックエンドは `GOOGLE_CLIENT_ID`（`infra/env/dev.tfvars` の値。秘密ではない）を付けて起動する。

## 4. 報告する

最後に、次の形で報告する。

```
## 画面の確認の結果

- 確認した時刻: 2026-10-18 14:05（平日ダイヤ）
- データ: 都営のみ / 他社のデータあり（extra の版）
- 確認したブランチ: <ブランチ名>

| 画面 | PC | スマホ | 確かめたこと | 結果 |
|---|---|---|---|---|
| 経路検索 | verify-journey-pc.png | verify-journey-sp.png | 浅草→新宿の経路、乗り換え、realtime=false の切り替え | OK |
| 駅 | ... | ... | ... | NG: 土休日のタブで次発が出ない |

### 見つかった問題
- （画面・手順・期待した結果・実際の結果。コンソールのエラーはそのまま引用する）

### 確かめられなかったこと
- （データが無くて確かめられなかった状態、送らなかったチャットなど）
```

問題が無くても「確かめられなかったこと」は省かずに書く。
