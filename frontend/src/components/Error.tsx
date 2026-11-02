import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

type Props = {
  message?: string;

  // 取り直す処理（useQuery の refetch など）。無ければページ全体を読み直す
  onRetry?: () => void;
  // 取り直している間（useQuery の isFetching）。ボタンを押せなくし、取り直していることを出す
  retrying?: boolean;
};

export default function Error({ message, onRetry, retrying }: Props) {
  return (
    <Alert variant="destructive">
      <AlertTitle>エラー</AlertTitle>

      <AlertDescription className="space-y-4">
        <p>{message ?? "データの取得に失敗しました。"}</p>

        <Button
          variant="outline"
          onClick={onRetry ?? (() => window.location.reload())}
          disabled={retrying}
        >
          {retrying ? "再読み込みしています" : "再読み込み"}
        </Button>
      </AlertDescription>
    </Alert>
  );
}
