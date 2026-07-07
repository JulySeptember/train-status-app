import { Link } from "react-router-dom";

import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-6">
      <h1 className="text-6xl font-bold">404</h1>

      <p>ページが見つかりません</p>

      <Link to="/">
        <Button>ホームへ戻る</Button>
      </Link>
    </div>
  );
}
