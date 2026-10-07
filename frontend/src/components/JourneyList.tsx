import { Link } from "react-router-dom";
import { ArrowDown, MapPin, Repeat } from "lucide-react";

import { type Journey, type JourneyLeg } from "@/types";

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
          ? "rounded border border-[#30363d] px-1.5 py-0.5 text-xs text-gray-400"
          : "rounded border border-[#f0883e]/60 bg-[#f0883e]/15 px-1.5 py-0.5 text-xs font-semibold text-[#f0883e]"
      }
    >
      {leg.trainTypeName}
    </span>
  );
}

function Stop({ time, id, name }: { time: string; id: string; name: string }) {
  return (
    <div className="flex items-baseline gap-4">
      <p className="w-14 shrink-0 text-xl font-bold text-white">{time}</p>

      <Link
        to={`/stations/${encodeURIComponent(id)}`}
        className="flex items-center gap-1.5 font-medium text-white hover:text-[#58a6ff]"
      >
        <MapPin size={14} className="text-gray-400" />
        {name}
      </Link>
    </div>
  );
}

function Leg({ leg }: { leg: JourneyLeg }) {
  return (
    <div className="space-y-2">
      <Stop time={leg.departureTime} id={leg.from} name={leg.fromName} />

      <div className="ml-6 flex gap-4 border-l-2 border-[#58a6ff]/60 py-2 pl-12">
        <div className="space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold text-white">{leg.railwayName}</span>

            <TrainType leg={leg} />

            {leg.destinationName && (
              <span className="text-gray-300">{leg.destinationName}行</span>
            )}
          </div>

          <p className="text-sm text-gray-400">
            {duration(leg.departureTime, leg.arrivalTime)}乗車・
            <Link
              to={`/trains/${encodeURIComponent(leg.train)}`}
              className="hover:text-[#58a6ff]"
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

function Transfer({ prev, next }: { prev: JourneyLeg; next: JourneyLeg }) {
  // 同じ駅（同じ路線）での乗り継ぎか、別の路線の駅への乗り換えか
  const label =
    prev.to === next.from
      ? `${prev.toName}で乗り継ぎ`
      : `${next.fromName}（${next.railwayName}）へ乗り換え`;

  return (
    <div className="ml-6 flex items-center gap-2 border-l-2 border-dashed border-[#30363d] py-3 pl-12 text-sm text-gray-400">
      <Repeat size={14} />
      {label}・待ち {duration(prev.arrivalTime, next.departureTime)}
    </div>
  );
}

function JourneyCard({ journey }: { journey: Journey }) {
  return (
    <div className="overflow-hidden rounded-xl border border-[#30363d] bg-[#0d1117]">
      <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1 border-b border-[#30363d] px-5 py-4">
        <p className="flex items-center gap-2 text-2xl font-bold text-white">
          {journey.departureTime}
          <ArrowDown size={18} className="-rotate-90 text-gray-400" />
          {journey.arrivalTime}
        </p>

        <p className="text-gray-300">
          {duration(journey.departureTime, journey.arrivalTime)}
        </p>

        <span className="rounded-full bg-[#1f6feb]/20 px-2 py-0.5 text-xs text-[#58a6ff]">
          {journey.transfers === 0
            ? "乗り換えなし"
            : `乗り換え${journey.transfers}回`}
        </span>
      </div>

      <div className="px-5 py-4">
        {journey.legs.map((leg, i) => (
          <div key={`${leg.train}-${leg.from}`}>
            {i > 0 && <Transfer prev={journey.legs[i - 1]} next={leg} />}
            <Leg leg={leg} />
          </div>
        ))}
      </div>
    </div>
  );
}

export default function JourneyList({ journeys }: { journeys: Journey[] }) {
  return (
    <div className="space-y-4">
      {journeys.map((journey) => (
        <JourneyCard
          key={`${journey.transfers}-${journey.departureTime}`}
          journey={journey}
        />
      ))}
    </div>
  );
}
