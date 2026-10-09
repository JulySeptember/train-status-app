import { Link } from "react-router-dom";
import {
  ArrowDown,
  ArrowRightLeft,
  Footprints,
  MapPin,
  Repeat,
} from "lucide-react";

import { type Journey, type JourneyLeg } from "@/types";

import RailwayBadge from "@/components/RailwayBadge";

import { FALLBACK_RAILWAY_COLOR, useRailway } from "@/lib/railways";

// "HH:MM" を運行日の0時からの分にする（3時前は翌日として24時間を足す）
function minutes(time: string) {
  const [h, m] = time.split(":").map(Number);
  return (h < 3 ? h + 24 : h) * 60 + m;
}

function duration(from: string, to: string) {
  const total = minutes(to) - minutes(from);
  const h = Math.floor(total / 60);
  const m = total % 60;

  return h > 0 ? `${h}時間${m}分` : `${m}分`;
}

function TrainType({ leg }: { leg: JourneyLeg }) {
  if (!leg.trainTypeName) {
    return null;
  }

  // 各停（普通）以外の種別は色を変えて目立たせる（Timetable.tsx と同じ）
  const isLocal = leg.trainType.endsWith(".Local");

  return (
    <span
      className={
        isLocal
          ? "rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground"
          : "rounded border border-warning/60 bg-warning/15 px-1.5 py-0.5 text-xs font-semibold text-warning"
      }
    >
      {leg.trainTypeName}
    </span>
  );
}

function Stop({ time, id, name }: { time: string; id: string; name: string }) {
  return (
    <div className="flex items-baseline gap-4">
      <p className="w-14 shrink-0 text-xl font-bold text-foreground">{time}</p>

      <Link
        to={`/stations/${encodeURIComponent(id)}`}
        className="flex items-center gap-1.5 font-medium text-foreground hover:text-brand"
      >
        <MapPin size={14} className="text-muted-foreground" />
        {name}
      </Link>
    </div>
  );
}

function Leg({ leg }: { leg: JourneyLeg }) {
  const railway = useRailway(leg.railway);

  return (
    <div className="space-y-2">
      <Stop time={leg.departureTime} id={leg.from} name={leg.fromName} />

      {/* 乗車区間の線は路線の色にする */}
      <div
        className="ml-6 flex gap-4 border-l-4 py-2 pl-12"
        style={{ borderColor: railway?.color || FALLBACK_RAILWAY_COLOR }}
      >
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <RailwayBadge railway={railway} className="size-6" />

            <span className="font-semibold text-foreground">
              {leg.railwayName}
            </span>

            <TrainType leg={leg} />

            {leg.destinationName && (
              <span className="text-foreground/80">
                {leg.destinationName}行
              </span>
            )}

            {leg.delayMinutes > 0 && (
              <span className="rounded border border-destructive/60 bg-destructive/15 px-1.5 py-0.5 text-xs font-semibold text-destructive">
                約{leg.delayMinutes}分遅れ
              </span>
            )}
          </div>

          <p className="text-sm text-muted-foreground">
            {duration(leg.departureTime, leg.arrivalTime)}乗車・
            <Link
              to={`/trains/${encodeURIComponent(leg.train)}`}
              className="hover:text-brand"
            >
              列車番号 {leg.trainNumber}
            </Link>
          </p>
        </div>
      </div>

      <Stop time={leg.arrivalTime} id={leg.to} name={leg.toName} />
    </div>
  );
}

// 直通運転の境目。同じ列車に乗ったまま、次の事業者の路線に入る
function Through({ prev, next }: { prev: JourneyLeg; next: JourneyLeg }) {
  const stop =
    next.departureTime === prev.arrivalTime
      ? ""
      : `・${duration(prev.arrivalTime, next.departureTime)}停車`;

  return (
    <div className="ml-6 flex items-center gap-2 border-l-2 border-border py-3 pl-12 text-sm text-muted-foreground">
      <ArrowRightLeft size={14} />
      {prev.toName}からそのまま{next.railwayName}に直通{stop}
    </div>
  );
}

function Transfer({ prev, next }: { prev: JourneyLeg; next: JourneyLeg }) {
  // 同じ駅（同じ路線）での乗り継ぎか、別の路線の駅への乗り換えか
  const label =
    prev.to === next.from
      ? `${prev.toName}で乗り継ぎ`
      : `${next.fromName}（${next.railwayName}）へ乗り換え`;

  return (
    <div className="ml-6 flex items-center gap-2 border-l-2 border-dashed border-border py-3 pl-12 text-sm text-muted-foreground">
      <Repeat size={14} />
      {label}・待ち {duration(prev.arrivalTime, next.departureTime)}
    </div>
  );
}

// 出発駅から最初に乗る駅まで・最後に降りる駅から到着駅まで歩く区間（例: 東京を指定して大手町から乗る）
function Walk({ label }: { label: string }) {
  return (
    <div className="ml-6 flex items-center gap-2 border-l-2 border-dotted border-border py-3 pl-12 text-sm text-muted-foreground">
      <Footprints size={14} />
      {label}
    </div>
  );
}

type Names = {
  // 経路検索で指定した出発駅・到着駅の名前。分からなければ「出発駅」「到着駅」と出す
  origin?: string;
  destination?: string;
};

function JourneyCard({
  journey,
  origin = "出発駅",
  destination = "到着駅",
}: { journey: Journey } & Names) {
  const first = journey.legs[0];
  const last = journey.legs[journey.legs.length - 1];

  return (
    <div className="overflow-hidden rounded-xl border border-border bg-background">
      <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1 border-b border-border px-5 py-4">
        <p className="flex items-center gap-2 text-2xl font-bold text-foreground">
          {journey.departureTime}
          <ArrowDown size={18} className="-rotate-90 text-muted-foreground" />
          {journey.arrivalTime}
        </p>

        <p className="text-foreground/80">
          {duration(journey.departureTime, journey.arrivalTime)}
        </p>

        <span className="rounded-full bg-primary/20 px-2 py-0.5 text-xs text-brand">
          {journey.transfers === 0
            ? "乗り換えなし"
            : `乗り換え${journey.transfers}回`}
        </span>
      </div>

      <div className="px-5 py-4">
        {journey.walkBeforeMinutes > 0 && (
          <Walk
            label={`${origin}から${first.fromName}まで徒歩${journey.walkBeforeMinutes}分`}
          />
        )}

        {journey.legs.map((leg, i) => (
          <div key={`${leg.train}-${leg.from}`}>
            {i > 0 &&
              (leg.through ? (
                <Through prev={journey.legs[i - 1]} next={leg} />
              ) : (
                <Transfer prev={journey.legs[i - 1]} next={leg} />
              ))}
            <Leg leg={leg} />
          </div>
        ))}

        {journey.walkAfterMinutes > 0 && (
          <Walk
            label={`${last.toName}から${destination}まで徒歩${journey.walkAfterMinutes}分`}
          />
        )}
      </div>
    </div>
  );
}

export default function JourneyList({
  journeys,
  ...names
}: { journeys: Journey[] } & Names) {
  return (
    <div className="space-y-4">
      {journeys.map((journey) => (
        <JourneyCard
          key={`${journey.transfers}-${journey.departureTime}`}
          journey={journey}
          {...names}
        />
      ))}
    </div>
  );
}
