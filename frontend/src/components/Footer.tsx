import { Link } from "react-router-dom";

export default function Footer() {
  return (
    <footer className="mt-16 border-t">
      <div className="container mx-auto flex items-center justify-between py-6 text-sm text-muted-foreground">
        <p>© 2026 Train Status App</p>

        <Link
          to="/license"
          className="underline underline-offset-4 hover:text-foreground"
        >
          データ提供・ライセンス
        </Link>
      </div>
    </footer>
  );
}
