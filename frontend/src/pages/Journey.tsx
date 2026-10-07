import { useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ArrowUpDown } from "lucide-react";

import { api } from "@/api";
import { type JourneyQuery, type Station } from "@/types";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import StationSelect from "@/components/StationSelect";
import JourneyList from "@/components/JourneyList";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

import { cn } from "@/lib/utils";

type Mode = "now" | "departAt" | "arriveBy";

const modes: { value: Mode; label: string }[] = [
  { value: "now", label: "今すぐ" },
  { value: "departAt", label: "出発" },
  { value: "arriveBy", label: "到着" },
];

// 検索条件は URL に持たせる（共有・再読み込みしても同じ結果になるように）
function readQuery(params: URLSearchParams): JourneyQuery | null {
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
  };
}

// 同じ名前の駅（新宿・春日など）は、経路検索では1つの駅として扱われるので、選択肢も1つにまとめる
function uniqueByName(stations: Station[]) {
  const seen = new Set<string>();

  return stations.filter((s) => {
    if (seen.has(s.name)) {
      return false;
    }
    seen.add(s.name);
    return true;
  });
}

export default function Journey() {
  const [params, setParams] = useSearchParams();
  const query = readQuery(params);

  const [fromId, setFromId] = useState(query?.from ?? "");
  const [toId, setToId] = useState(query?.to ?? "");
  const [mode, setMode] = useState<Mode>(
    query?.arriveBy ? "arriveBy" : query?.departAt ? "departAt" : "now",
  );
  const [time, setTime] = useState(query?.arriveBy ?? query?.departAt ?? "");

  const stations = useQuery({
    queryKey: ["stations"],
    queryFn: api.getAllStations,
  });

  const options = useMemo(
    () => uniqueByName(stations.data ?? []),
    [stations.data],
  );

  // URL の駅IDが、まとめた選択肢にない側の駅（例: 大江戸線の春日）でも選択済みとして表示する
  const selected = (id: string) => {
    const name = stations.data?.find((s) => s.id === id)?.name;
    return options.find((s) => s.name === name)?.id ?? id;
  };

  const journeys = useQuery({
    queryKey: ["journeys", query],
    queryFn: () => api.searchJourneys(query!),
    enabled: query !== null,
  });

  if (stations.isPending) {
    return <Loading />;
  }

  if (stations.error) {
    return <Error />;
  }

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
    setParams(next);
  };

  return (
    <div className="space-y-8">
      <h1 className="text-3xl font-bold">経路検索</h1>

      <div className="space-y-4 rounded-xl border border-[#30363d] bg-[#161b22] p-5">
        <div className="grid items-center gap-3 md:grid-cols-[1fr_auto_1fr]">
          <StationSelect
            stations={options}
            value={selected(fromId)}
            onChange={setFromId}
            label="出発駅"
          />

          <Button
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
          />
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <div className="flex rounded-lg border border-[#30363d] p-0.5">
            {modes.map((m) => (
              <button
                key={m.value}
                type="button"
                onClick={() => setMode(m.value)}
                className={cn(
                  "rounded-md px-3 py-1.5 text-sm transition",
                  mode === m.value
                    ? "bg-[#1f6feb] text-white"
                    : "text-gray-300 hover:bg-[#21262d]",
                )}
              >
                {m.label}
              </button>
            ))}
          </div>

          {mode !== "now" && (
            <Input
              type="time"
              value={time}
              onChange={(e) => setTime(e.target.value)}
              className="w-32"
              aria-label={mode === "departAt" ? "出発時刻" : "到着時刻"}
            />
          )}

          <Button disabled={!canSearch} onClick={search} className="ml-auto">
            検索
          </Button>
        </div>

        <p className="text-xs text-gray-400">
          本日のダイヤで検索します。0時〜2時台は前日の深夜として扱います。時刻表どおりの結果で、遅延は反映していません。
        </p>
      </div>

      {query && journeys.isPending && <Loading />}

      {query && journeys.error && (
        <div className="rounded-lg border p-6 text-center">
          <p className="text-lg font-medium">経路を検索できませんでした。</p>

          <p className="mt-2 text-sm text-muted-foreground">
            駅や時刻の指定を確かめて、もう一度検索してください。
          </p>
        </div>
      )}

      {query && journeys.data?.journeys.length === 0 && (
        <div className="rounded-lg border p-6 text-center">
          <p className="text-lg font-medium">経路が見つかりませんでした。</p>

          <p className="mt-2 text-sm text-muted-foreground">
            終電を過ぎているか、乗り換え3回までではたどり着けない可能性があります。
          </p>
        </div>
      )}

      {query && journeys.data && journeys.data.journeys.length > 0 && (
        <JourneyList journeys={journeys.data.journeys} />
      )}
    </div>
  );
}
