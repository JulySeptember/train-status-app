import { Link } from "react-router-dom";

export default function Footer() {
  return (
    <footer className="border-t border-border bg-card">
      <div className="mx-auto flex max-w-7xl items-center justify-center px-6 py-5">
        <div className="flex flex-wrap items-center justify-center gap-4 text-sm text-muted-foreground">
          <span>東京都交通局オープンデータ</span>

          <a
            href="https://creativecommons.org/licenses/by/4.0/deed.ja"
            target="_blank"
            rel="noopener noreferrer"
            className="text-primary hover:underline"
          >
            CC BY 4.0
          </a>

          <Link to="/license" className="text-primary hover:underline">
            ライセンス
          </Link>
        </div>
      </div>
    </footer>
  );
}
