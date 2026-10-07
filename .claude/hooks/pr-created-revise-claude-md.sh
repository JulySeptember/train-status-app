#!/usr/bin/env bash
# PostToolUse(Bash) フック:
# `gh pr create` の実行後に、セッションの学びを CLAUDE.md へ反映するようモデルへ促す。

set -euo pipefail

input="$(cat)"

command="$(jq -r '.tool_input.command // ""' <<<"$input")"

# `cd repo && gh pr create ...` のような複合コマンドにも対応するため、
# &&・||・; で区切った各セグメントの先頭が `gh pr create` かを判定する
is_pr_create=false

while IFS= read -r segment; do
  segment="${segment#"${segment%%[![:space:]]*}"}"

  if [[ "$segment" == "gh pr create"* ]]; then
    is_pr_create=true
    break
  fi
done < <(sed -E 's/(&&|\|\||;)/\n/g' <<<"$command")

if [[ "$is_pr_create" != true ]]; then
  exit 0
fi

read -r -d '' context <<'EOF' || true
PR が作成されました。このブランチでの作業を振り返り、次の観点で CLAUDE.md に反映すべき内容があるか判断してください。

1. セッションの重要な学び: コードを複数読まないと分からなかった設計・データ仕様・外部 API の癖など
2. 次回の再発防止点: 今回つまずいた点、誤った前提、やり直しになった作業、環境・ツール由来の問題
3. この repo に反映すべき CLAUDE.md の変更案: 追記・修正・削除（古くなった記述も含む）

反映すべき内容がある場合は `/claude-md-management:revise-claude-md`（/revise-claude-md）を実行し、変更案をユーザーに提示してから CLAUDE.md を更新してください。
コードやコミット履歴から自明なこと、このセッションだけに関係することは書かないでください。
反映すべき内容がない場合は、その旨を一言伝えるだけで構いません。
EOF

jq -n --arg ctx "$context" '{
  hookSpecificOutput: {
    hookEventName: "PostToolUse",
    additionalContext: $ctx
  }
}'
