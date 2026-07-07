import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

type Props = {
  message?: string;
};

export default function Error({ message }: Props) {
  return (
    <Alert variant="destructive">
      <AlertTitle>エラー</AlertTitle>

      <AlertDescription className="space-y-4">
        <p>{message ?? "データの取得に失敗しました。"}</p>

        <Button variant="outline" onClick={() => window.location.reload()}>
          再読み込み
        </Button>
      </AlertDescription>
    </Alert>
  );
}
