# frontend/CLAUDE.md

React の SPA。ルートの CLAUDE.md の「守ること」も合わせて守る。

## 構成

- React 19 + Vite + Tailwind v4 + shadcn/ui（`components/ui`）、TanStack Query、React Router。パスエイリアス `@` は `src/` を指す
- API 呼び出しは `src/api.ts`、レスポンス型は `src/types.ts` に集約している。バックエンドの DTO を変えたらこの2ファイルと `e2e/data.ts` も合わせる
- ODPT の ID を日本語ラベルに変換する処理は `src/lib/odpt.ts`
- ID は `odpt.Station:...` のように `:` や `.` を含むので、URL に入れるときは `encodeURIComponent` する
- `components/ui` は base-ui ベース。`PopoverTrigger` などのトリガーに `Button` を使うときは、子要素にせず `render={<Button ... />}` で渡す（子にすると button が入れ子になる）
- `components/ui` は shadcn の生成物で書式が違う（行末のセミコロンが無い）。Prettier は変えたファイルだけにかけ、`npx prettier --write src` のようにまとめてかけない
- 色は `index.css` のトークン（`primary`・`brand`・`warning`・`destructive-foreground` など）で指定し、色コードを直接書かない。路線の色・記号は `GET /api/routes`（ODPT の `odpt:color`・`odpt:lineCode`）を `lib/railways.ts` の `useRailways` / `useRailway` で引く

## テスト

テストは Playwright の E2E テスト（`e2e/`）だけ。`/api` は `e2e/mock.ts` の `page.route` で固定の応答（`e2e/data.ts`、`src/types.ts` の型付き）を返すので、バックエンドは起動しない。登録していない API が呼ばれるとテストが失敗する。現在時刻は `NOW`（平日の朝）に固定している。DTO を変えたら `data.ts` も合わせる（`tsc -b` で型のずれに気づける）。対象は運行情報・路線・駅（時刻表）・列車・経路検索で、AI チャット・ログイン・通知・スマホの幅は無い。
