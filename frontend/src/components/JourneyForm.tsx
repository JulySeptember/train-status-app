import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowUpDown, Search } from "lucide-react";

import { api } from "@/api";
import { type JourneyQuery, type Railway, type StationSummary } from "@/types";

import StationSelect, { type StationOption } from "@/components/StationSelect";
import RailwayBadge from "@/components/RailwayBadge";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { uniqueBadges, useRailways } from "@/lib/railways";

type Mode = "now" | "departAt" | "arriveBy";

const modes: { value: Mode; label: string }[] = [
  { value: "now", label: "今すぐ" },
  { value: "departAt", label: "出発" },
  { value: "arriveBy", label: "到着" },
];

// 検索条件は URL に持たせる（共有・再読み込みしても同じ結果になるように）
export function readJourneyQuery(params: URLSearchParams): JourneyQuery | null {
  const from = params.get("from");
  const to = params.get("to");

  if (!from || !to) {
    return null;
  }

  return {
    from,
    to,
    departAt: params.get("departAt") ?? undefined,
    arriveBy: params.get("arriveBy") ?? undefined,
    realtime: params.get("realtime") !== "false",
  };
}

// 経路検索で1つの駅として扱われる駅（近くにある同じ名前の駅。新宿・春日など）は、選択肢も1つにまとめる。
// 選択肢の ID は、まとめた駅の代表（journeyStation）。経路検索に使えない駅は選べない選択肢にする
function journeyOptions(
  stations: StationSummary[],
  railways: Railway[],
): StationOption[] {
  const groups = new Map<string, StationSummary[]>();

  for (const s of stations) {
    const key = s.journeyStation || s.id;
    groups.set(key, [...(groups.get(key) ?? []), s]);
  }

  const railwayById = new Map(railways.map((r) => [r.id, r]));

  return [...groups].map(([id, items]) => {
    const badges = uniqueBadges(
      [...new Set(items.map((s) => s.railwayId))].flatMap(
        (id) => railwayById.get(id) ?? [],
      ),
    );

    return {
      id,
      name: items[0].name,
      // 同じ名前でも離れた駅（早稲田の東西線と荒川線など）は別の選択肢になるので、路線の記号で見分ける
      detail: (
        <span className="flex flex-wrap gap-1">
          {badges.map((r) => (
            <RailwayBadge
              key={r.id}
              railway={r}
              className="size-5 text-[10px]"
            />
          ))}
        </span>
      ),
      disabledReason: items[0].journeyStation ? undefined : "経路検索は未対応",
    };
  });
}

type Props = {
  // URL から読んだ検索条件。入力欄の初期値にする
  initial?: JourneyQuery | null;

  // 検索条件を URL のパラメーターにして返す
  onSearch(params: URLSearchParams): void;
};

export default function JourneyForm({ initial, onSearch }: Props) {
  const [fromId, setFromId] = useState(initial?.from ?? "");
  const [toId, setToId] = useState(initial?.to ?? "");
  const [mode, setMode] = useState<Mode>(
    initial?.arriveBy ? "arriveBy" : initial?.departAt ? "departAt" : "now",
  );
  const [time, setTime] = useState(
    initial?.arriveBy ?? initial?.departAt ?? "",
  );
  const [realtime, setRealtime] = useState(initial?.realtime ?? true);

  const stations = useQuery({
    queryKey: ["stations"],
    queryFn: api.getAllStations,
  });

  const railways = useRailways();

  const options = useMemo(
    () => journeyOptions(stations.data ?? [], railways.data ?? []),
    [stations.data, railways.data],
  );

  // URL の駅IDが、まとめた選択肢の代表でない駅（例: 大江戸線の春日）でも選択済みとして表示する
  const selected = (id: string) =>
    stations.data?.find((s) => s.id === id)?.journeyStation || id;

  const canSearch =
    fromId !== "" &&
    toId !== "" &&
    selected(fromId) !== selected(toId) &&
    (mode === "now" || time !== "");

  const search = () => {
    const next = new URLSearchParams({ from: fromId, to: toId });
    if (mode !== "now") {
      next.set(mode, time);
    }
    if (!realtime) {
      next.set("realtime", "false");
    }
    onSearch(next);
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (canSearch) {
          search();
        }
      }}
      className="space-y-4"
    >
      <div className="grid items-center gap-3 md:grid-cols-[1fr_auto_1fr]">
        <StationSelect
          stations={options}
          value={selected(fromId)}
          onChange={setFromId}
          label="出発駅"
          disabled={stations.isPending}
        />

        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label="出発駅と到着駅を入れ替える"
          className="justify-self-center"
          onClick={() => {
            setFromId(toId);
            setToId(fromId);
          }}
        >
          <ArrowUpDown className="md:-rotate-90" />
        </Button>

        <StationSelect
          stations={options}
          value={selected(toId)}
          onChange={setToId}
          label="到着駅"
          disabled={stations.isPending}
        />
      </div>

      {stations.error && (
        <p className="text-sm text-destructive-foreground">
          駅の一覧を取得できませんでした。再読み込みしてください。
        </p>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <Tabs value={mode} onValueChange={(v) => setMode(v as Mode)}>
          <TabsList aria-label="時刻の指定">
            {modes.map((m) => (
              <TabsTrigger key={m.value} value={m.value} className="px-3">
                {m.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {mode !== "now" && (
          <Input
            type="time"
            value={time}
            onChange={(e) => setTime(e.target.value)}
            className="w-32"
            aria-label={mode === "departAt" ? "出発時刻" : "到着時刻"}
          />
        )}

        <label className="flex items-center gap-2 text-sm text-foreground/80">
          <input
            type="checkbox"
            checked={realtime}
            onChange={(e) => setRealtime(e.target.checked)}
            className="size-4 accent-primary"
          />
          遅延・運転見合わせを反映
        </label>

        <Button type="submit" disabled={!canSearch} className="ml-auto px-4">
          <Search />
          検索
        </Button>
      </div>

      <p className="text-xs text-muted-foreground">
        本日のダイヤで検索します。0時〜2時台は前日の深夜として扱います。
        {realtime
          ? "現在の遅れ（路線・方向ごとの見込み）を1時間以内に出る列車の時刻に足し、運転を見合わせている路線は使いません。"
          : "遅延・運転見合わせは反映せず、時刻表どおりに検索します。"}
      </p>
    </form>
  );
}
