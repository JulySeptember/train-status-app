import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

type Props = {
  message?: string;

  // 取り直す処理（useQuery の refetch など）。無ければページ全体を読み直す
  onRetry?: () => void;
};

export default function Error({ message, onRetry }: Props) {
  return (
    <Alert variant="destructive">
      <AlertTitle>エラー</AlertTitle>

      <AlertDescription className="space-y-4">
        <p>{message ?? "データの取得に失敗しました。"}</p>

        <Button
          variant="outline"
          onClick={onRetry ?? (() => window.location.reload())}
        >
          再読み込み
        </Button>
      </AlertDescription>
    </Alert>
  );
}
