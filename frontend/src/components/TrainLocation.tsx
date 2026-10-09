import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowUp,
  ChevronRight,
  Clock,
  MapPin,
  TrainFront,
} from "lucide-react";

import { api } from "@/api";
import {
  type Railway,
  type Station,
  type TrainLocation as TrainLocationType,
} from "@/types";

import RailwayBadge from "@/components/RailwayBadge";

import { Button } from "@/components/ui/button";
import { directionLabel } from "@/lib/odpt";
import { FALLBACK_RAILWAY_COLOR, useRailway } from "@/lib/railways";
import { cn } from "@/lib/utils";

type Props = {
  train: TrainLocationType;
};

// 路線図で、列車の前後に見せる駅の数（「全駅を表示」で全駅にする）
const NEARBY_STATIONS = 3;

function DelayBadge({ train }: Props) {
  // 荒川線は遅れが配信されないので、定刻とは言わない
  if (!train.delayAvailable) {
    return (
      <span className="rounded-full bg-muted px-2.5 py-1 text-sm text-muted-foreground">
        遅れの情報なし
      </span>
    );
  }

  if (train.delay <= 0) {
    return (
      <span className="rounded-full bg-brand/15 px-2.5 py-1 text-sm font-semibold text-brand">
        定刻
      </span>
    );
  }

  return (
    <span className="rounded-full bg-warning/15 px-2.5 py-1 text-sm font-semibold text-warning">
      約{Math.max(1, Math.round(train.delay / 60))}分遅れ
    </span>
  );
}

// 位置情報の配信時刻を「20:12:26」の形にする。読めない値なら空にする
function formatTime(value: string) {
  const date = new Date(value);

  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return date.toLocaleTimeString("ja-JP", { timeZone: "Asia/Tokyo" });
}

function TrainMarker() {
  return (
    <span
      aria-hidden
      className="absolute top-1/2 left-0 flex size-8 -translate-y-1/2 items-center justify-center rounded-full bg-primary text-primary-foreground ring-4 ring-brand/25"
    >
      <TrainFront size={16} />
    </span>
  );
}

// 駅をつなぐ縦の線。路線の端の駅では、外側の半分を描かない
function Line({
  color,
  first,
  last,
}: {
  color: string;
  first?: boolean;
  last?: boolean;
}) {
  return (
    <span
      aria-hidden
      className={cn(
        "absolute left-3.5 w-1",
        first ? "top-1/2" : "top-0",
        last ? "bottom-1/2" : "bottom-0",
      )}
      style={{ backgroundColor: color }}
    />
  );
}

type Row =
  | { kind: "station"; index: number; station: Station; current: boolean }
  | { kind: "train" };

// 路線の駅を路線の順に並べ、列車の位置を書き込む。
// 停車中はその駅の行に、走行中は前後の駅の行のあいだに列車を置く
function RouteDiagram({
  train,
  railway,
  stations,
}: Props & { railway?: Railway; stations: Station[] }) {
  const [showAll, setShowAll] = useState(false);

  const color = railway?.color || FALLBACK_RAILWAY_COLOR;

  const fromIndex = stations.findIndex((s) => s.id === train.fromStationId);
  const toIndex = stations.findIndex((s) => s.id === train.toStationId);

  // 大江戸線の環状部（新宿 → 都庁前）のように、隣り合わない駅のあいだを走ることもある
  const adjacent = toIndex >= 0 && Math.abs(toIndex - fromIndex) === 1;

  const first = showAll ? 0 : Math.max(0, fromIndex - NEARBY_STATIONS);
  const last = showAll
    ? stations.length - 1
    : Math.min(stations.length - 1, fromIndex + NEARBY_STATIONS);

  const rows: Row[] = [];

  for (let i = first; i <= last; i++) {
    rows.push({
      kind: "station",
      index: i,
      station: stations[i],
      current: train.stopped && i === fromIndex,
    });

    // 走行中の列車は、路線の順で手前にある駅の行のすぐ下に置く
    const trainAfter = adjacent ? Math.min(fromIndex, toIndex) : fromIndex;
    if (!train.stopped && i === trainAfter) {
      rows.push({ kind: "train" });
    }
  }

  const label = adjacent
    ? "走行中"
    : train.toStation
      ? `${train.toStation}へ向かっています`
      : "走行中";

  const Arrow = toIndex > fromIndex ? ArrowDown : ArrowUp;

  return (
    <div className="space-y-2">
      <ol>
        {rows.map((row) => {
          if (row.kind === "train") {
            return (
              <li
                key="train"
                className="relative flex min-h-12 items-center pl-10"
              >
                <Line color={color} />
                <TrainMarker />

                <span className="flex items-center gap-1 px-3 text-sm font-semibold text-brand">
                  {adjacent && <Arrow size={16} />}
                  {label}
                </span>
              </li>
            );
          }

          const { index, station, current } = row;

          return (
            <li
              key={station.id}
              className="relative flex min-h-11 items-center pl-10"
            >
              <Line
                color={color}
                first={index === 0}
                last={index === stations.length - 1}
              />

              {current ? (
                <TrainMarker />
              ) : (
                <span
                  aria-hidden
                  className="absolute top-1/2 left-2.5 size-3 -translate-y-1/2 rounded-full border-[3px] bg-card"
                  style={{ borderColor: color }}
                />
              )}

              <Link
                to={`/stations/${encodeURIComponent(station.id)}`}
                className={cn(
                  "flex flex-1 items-center justify-between gap-2 rounded-lg px-3 py-2 transition hover:bg-muted",
                  current && "bg-brand/10 font-semibold",
                )}
              >
                <span className="flex items-center gap-2">
                  {station.name}

                  {current && (
                    <span className="rounded-full bg-brand/20 px-2 py-0.5 text-xs text-brand">
                      停車中
                    </span>
                  )}
                </span>

                <ChevronRight
                  size={16}
                  className="shrink-0 text-muted-foreground"
                />
              </Link>
            </li>
          );
        })}
      </ol>

      {stations.length > NEARBY_STATIONS * 2 + 1 && (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setShowAll((v) => !v)}
          className="text-muted-foreground"
        >
          {showAll ? "前後の駅だけを表示" : `全${stations.length}駅を表示`}
        </Button>
      )}
    </div>
  );
}

// 路線・方向・列車番号と、「普通 西馬込行」のような列車の名前
function TrainTitle({ train }: Props) {
  const railway = useRailway(train.railwayId);
  const isLocal = train.trainTypeId.endsWith(".Local");

  return (
    <>
      <p className="flex items-center gap-2 text-sm text-muted-foreground">
        <RailwayBadge railway={railway} className="size-6" />
        {[
          train.railway,
          train.railDirection && directionLabel(train.railDirection),
          `列車番号 ${train.trainNumber}`,
        ]
          .filter(Boolean)
          .join("・")}
      </p>

      <h1 className="flex flex-wrap items-center gap-2 text-2xl font-bold">
        {train.trainType && (
          <span
            className={
              isLocal
                ? "rounded border border-border px-2 py-0.5 text-base font-medium text-muted-foreground"
                : "rounded border border-warning/60 bg-warning/15 px-2 py-0.5 text-base font-semibold text-warning"
            }
          >
            {train.trainType}
          </span>
        )}
        {train.destination
          ? `${train.destination}行`
          : `列車番号 ${train.trainNumber}`}
      </h1>
    </>
  );
}

// 位置が配信されていない列車。本日のダイヤから、出発前か運行を終えたかを出す
export function TrainNotRunning({ train }: Props) {
  const { notRunning, scheduledStationId, scheduledStation, scheduledTime } =
    train;
  // 直通運転の列車は他社の線内で走り続けるので、どの事業者の線内かを書く
  const operatorName = useRailway(train.railwayId)?.operatorName;

  const heading =
    notRunning === "beforeDeparture"
      ? "まだ出発していません"
      : notRunning === "finished"
        ? `${operatorName ? `${operatorName}の` : ""}線内の運行を終えました`
        : "位置を取得できません";

  return (
    <div className="max-w-2xl space-y-6">
      <section className="space-y-4 rounded-xl border bg-card p-5">
        {train.railwayId ? (
          <TrainTitle train={train} />
        ) : (
          <h1 className="text-2xl font-bold">列車番号 {train.trainNumber}</h1>
        )}

        <div className="space-y-2 rounded-lg bg-muted/50 p-4">
          <p className="flex items-center gap-1.5 font-semibold">
            <TrainFront size={18} className="text-brand" />
            {heading}
          </p>

          {scheduledStationId && scheduledStation && scheduledTime && (
            <p className="text-2xl font-bold">
              <Link
                to={`/stations/${encodeURIComponent(scheduledStationId)}`}
                className="hover:text-brand"
              >
                {scheduledStation}
              </Link>
              <span className="ml-2">
                {scheduledTime}
                {notRunning === "finished" ? " 着" : " 発"}
              </span>
            </p>
          )}

          {/* 駅と時刻を出したときは、同じ内容の文章を繰り返さない。
              noData は「遅れてまだ出発していないことがある」を伝えるので出す */}
          {(!scheduledStation || notRunning === "noData") && (
            <p className="text-sm text-muted-foreground">{train.message}</p>
          )}
        </div>

        {(notRunning === "beforeDeparture" || notRunning === "noData") && (
          <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Clock size={14} />
            出発したら、自動で位置を表示します。
          </p>
        )}
      </section>
    </div>
  );
}

export default function TrainLocation({ train }: Props) {
  const railway = useRailway(train.railwayId);

  const stations = useQuery({
    queryKey: ["stations", train.railwayId],
    queryFn: () => api.getStations(train.railwayId),
    enabled: train.railwayId !== "",
    staleTime: Infinity,
  });

  const updatedAt = formatTime(train.updatedAt);

  const position = train.stopped
    ? `${train.fromStation}に停車中`
    : train.toStation
      ? `${train.fromStation} → ${train.toStation} を走行中`
      : `${train.fromStation}付近を走行中`;

  // 路線の駅の一覧に今いる駅が無いときは、路線図を出さずに文章だけにする
  const onRoute =
    stations.data?.some((s) => s.id === train.fromStationId) ?? false;

  return (
    <div className="max-w-2xl space-y-6">
      <section className="space-y-4 rounded-xl border bg-card p-5">
        <TrainTitle train={train} />

        <div className="flex flex-wrap items-center gap-3">
          <DelayBadge train={train} />

          <p className="flex items-center gap-1.5 font-medium">
            <MapPin size={18} className="text-brand" />
            {position}
          </p>
        </div>

        <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Clock size={14} />
          {updatedAt && `${updatedAt} 時点の位置。`}
          15秒ごとに自動で更新します。
        </p>
      </section>

      {onRoute && stations.data && (
        <section className="space-y-3 rounded-xl border bg-card p-5">
          <h2 className="text-lg font-semibold">現在位置</h2>

          <RouteDiagram
            train={train}
            railway={railway}
            stations={stations.data}
          />
        </section>
      )}
    </div>
  );
}
