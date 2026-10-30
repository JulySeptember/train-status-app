import { TriangleAlert } from "lucide-react";

import { type JourneySearch } from "@/types";

// 運転見合わせで除外した路線と、運行状況を取得できなかったこと（翌運行日の時刻で探して反映しなかったこと）を知らせる
export default function RealtimeNotice({
  data,
  realtime,
}: {
  data: JourneySearch;
  realtime: boolean;
}) {
  if (!realtime) {
    return null;
  }

  if (data.nextServiceDay) {
    return (
      <p className="rounded-lg border border-border px-4 py-3 text-sm text-foreground/80">
        今日の朝からの時刻表どおりの結果です（遅れや運転見合わせは反映していません）。
      </p>
    );
  }

  if (!data.delayApplied) {
    return (
      <p className="rounded-lg border border-border px-4 py-3 text-sm text-foreground/80">
        運行状況を取得できなかったため、時刻表どおりの結果です。
      </p>
    );
  }

  if (data.suspendedRailways.length === 0) {
    return null;
  }

  return (
    <p className="flex items-start gap-2 rounded-lg border border-destructive/60 bg-destructive/10 px-4 py-3 text-sm text-destructive-foreground">
      <TriangleAlert size={16} className="mt-0.5 shrink-0" />
      {data.suspendedRailways.map((r) => r.name || r.id).join("・")}
      は運転を見合わせているため、使わない経路を表示しています。
    </p>
  );
}
