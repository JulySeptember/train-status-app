import { Link } from "react-router-dom";

export default function Footer() {
  return (
    <footer className="border-t border-border bg-card">
      <div className="mx-auto max-w-6xl space-y-2 px-4 py-5 text-center text-sm text-muted-foreground sm:px-6">
        <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1">
          <span>出典: 公共交通オープンデータセンター（東京都交通局ほか）</span>

          <Link to="/license" className="text-brand hover:underline">
            データ提供・ライセンス
          </Link>
        </div>

        {/* 公共交通オープンデータチャレンジ限定ライセンス第10条3項・基本ライセンスで求められる表示 */}
        <p className="text-xs">
          情報の正確性・完全性は保証しません。データの提供元（公共交通オープンデータセンター・各事業者）へのお問い合わせはご遠慮ください。
        </p>
      </div>
    </footer>
  );
}
