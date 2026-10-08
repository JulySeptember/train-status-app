import { Link } from "react-router-dom";

export default function Footer() {
  return (
    <footer className="border-t border-border bg-card">
      <div className="mx-auto flex max-w-6xl items-center justify-center px-4 py-5 sm:px-6">
        <div className="flex flex-wrap items-center justify-center gap-4 text-sm text-muted-foreground">
          <span>東京都交通局オープンデータ</span>

          <a
            href="https://creativecommons.org/licenses/by/4.0/deed.ja"
            target="_blank"
            rel="noopener noreferrer"
            className="text-brand hover:underline"
          >
            CC BY 4.0
          </a>

          <Link to="/license" className="text-brand hover:underline">
            ライセンス
          </Link>
        </div>
      </div>
    </footer>
  );
}
