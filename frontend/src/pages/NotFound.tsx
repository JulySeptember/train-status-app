import { Link } from "react-router-dom";

import PageTitle from "@/components/PageTitle";

import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-6">
      <PageTitle title="ページが見つかりません" />

      <h1 className="text-6xl font-bold">404</h1>

      <p>ページが見つかりません</p>

      {/* Link をそのまま Button として描く（中に入れると a の中に button が入る） */}
      <Button render={<Link to="/" />} nativeButton={false}>
        ホームへ戻る
      </Button>
    </div>
  );
}
