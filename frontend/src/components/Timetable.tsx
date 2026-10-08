import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Clock3, ArrowRight } from "lucide-react";

import {
  type DirectionTimetable,
  type Timetable as TimetableEntry,
} from "@/types";
import { nowServiceMinutes, timeToServiceMinutes } from "@/lib/time";

type Props = {
  weekday?: DirectionTimetable;
  saturday?: DirectionTimetable;
  holiday?: DirectionTimetable;
  saturdayHoliday?: DirectionTimetable;
  trainLocationAvailable: boolean;
};

type Tab = "weekday" | "saturday" | "holiday" | "saturdayHoliday";

// 次に発車する列車の位置。最終列車が出たあとは -1。
// 0時台・1時台の列車は運行日の最後に並ぶので、文字列ではなく運行日の経過分で比べる
function nextTrainIndex(trains: TimetableEntry[], now: number): number {
  let index = -1;
  let best = Infinity;

  trains.forEach((train, i) => {
    const minutes = timeToServiceMinutes(train.time);
    if (minutes !== null && minutes >= now && minutes < best) {
      best = minutes;
      index = i;
    }
  });

  return index;
}

// 本日のダイヤのとき、現在時刻（運行日の経過分）を30秒ごとに更新して返す
function useServiceNow(enabled: boolean): number | null {
  const [now, setNow] = useState(() => nowServiceMinutes());

  useEffect(() => {
    if (!enabled) {
      return;
    }

    const id = setInterval(() => setNow(nowServiceMinutes()), 30 * 1000);
    return () => clearInterval(id);
  }, [enabled]);

  return enabled ? now : null;
}

function TimetableCard({
  title,
  timetable,
  trainLocationAvailable,
}: {
  title: string;
  timetable?: DirectionTimetable;
  trainLocationAvailable: boolean;
}) {
  // 列車番号は平日・土休日ダイヤで使い回されるため、列車位置へのリンクは本日のダイヤに限る
  const linkable = trainLocationAvailable && !!timetable?.isToday;

  const now = useServiceNow(!!timetable?.isToday);
  const nextIndex =
    now !== null && timetable?.timetables
      ? nextTrainIndex(timetable.timetables, now)
      : -1;

  const nextRowRef = useRef<HTMLElement | null>(null);

  // 時刻表を開いたときと方面を切り替えたときに、次に発車する列車までスクロールする。
  // データの再取得や時刻の更新ではスクロールしない
  const scrollKey = timetable
    ? `${timetable.calendar}|${timetable.railDirection}`
    : "";

  useEffect(() => {
    const row = nextRowRef.current;

    // スマホ表示ではデスクトップ用の時刻表が、PC表示ではスマホ用が非表示のまま描画されている
    if (!row || row.getClientRects().length === 0) {
      return;
    }

    row.scrollIntoView({ block: "center" });
  }, [scrollKey]);

  return (
    <div className="overflow-hidden rounded-xl border border-[#30363d] bg-[#0d1117]">
      <div className="flex items-center gap-2 border-b border-[#30363d] px-5 py-4">
        <Clock3 size={18} className="text-[#2ea043]" />

        <h3 className="font-semibold text-white">{title}</h3>

        {timetable?.isToday && (
          <span className="rounded-full bg-[#1f6feb]/20 px-2 py-0.5 text-xs text-[#58a6ff]">
            本日
          </span>
        )}
      </div>

      <div className="divide-y divide-[#30363d]">
        {timetable?.timetables?.length ? (
          timetable.timetables.map((train, i) => {
            // 各停（普通）以外の種別は色を変えて目立たせる
            const isLocal = train.trainTypeId.endsWith(".Local");

            const isNext = i === nextIndex;
            const rowRef = isNext
              ? (el: HTMLElement | null) => {
                  nextRowRef.current = el;
                }
              : undefined;
            // scroll-mt-20: 固定ヘッダー（h-16）に行が隠れないようにする
            const rowClass = `scroll-mt-20 px-5 py-4 ${
              isNext ? "border-l-4 border-l-[#2ea043] bg-[#2ea043]/10" : ""
            }`;

            const content = (
              <div>
                <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                  <p className="text-2xl font-bold text-white">{train.time}</p>

                  {isNext && (
                    <span className="rounded-full bg-[#2ea043]/20 px-2 py-0.5 text-xs font-semibold text-[#3fb950]">
                      次発
                    </span>
                  )}

                  {train.trainType && (
                    <span
                      className={
                        isLocal
                          ? "rounded border border-[#30363d] px-1.5 py-0.5 text-xs text-gray-400"
                          : "rounded border border-[#f0883e]/60 bg-[#f0883e]/15 px-1.5 py-0.5 text-xs font-semibold text-[#f0883e]"
                      }
                    >
                      {train.trainType}
                    </span>
                  )}

                  {train.destination && (
                    <p className="font-medium text-white">
                      {train.destination}行
                    </p>
                  )}
                </div>

                <p className="mt-1 text-sm text-gray-400">
                  列車番号 {train.trainNumber}
                </p>
              </div>
            );

            if (!linkable) {
              return (
                <div
                  key={`${train.time}-${train.trainId}`}
                  ref={rowRef}
                  className={rowClass}
                >
                  {content}
                </div>
              );
            }

            return (
              <Link
                key={`${train.time}-${train.trainId}`}
                to={`/trains/${encodeURIComponent(train.trainId)}`}
                ref={rowRef}
                className={`flex items-center justify-between transition hover:bg-[#161b22] ${rowClass}`}
              >
                {content}

                <ArrowRight size={18} className="text-gray-500" />
              </Link>
            );
          })
        ) : (
          <div className="py-10 text-center text-gray-500">
            データがありません
          </div>
        )}
      </div>
    </div>
  );
}

export default function Timetable({
  weekday,
  saturday,
  holiday,
  saturdayHoliday,
  trainLocationAvailable,
}: Props) {
  // スマホ表示では、本日のダイヤのタブを最初に開く
  const [tab, setTab] = useState<Tab>(() => {
    const tabs: [Tab, DirectionTimetable | undefined][] = [
      ["weekday", weekday],
      ["saturday", saturday],
      ["holiday", holiday],
      ["saturdayHoliday", saturdayHoliday],
    ];
    return tabs.find(([, t]) => t?.isToday)?.[0] ?? "weekday";
  });

  if (saturdayHoliday) {
    return (
      <>
        {/* Mobile */}
        <div className="lg:hidden">
          <div className="mb-5 flex overflow-hidden rounded-xl border border-[#30363d] bg-[#161b22]">
            <button
              onClick={() => setTab("weekday")}
              className={`flex-1 py-3 text-sm font-medium transition ${
                tab === "weekday"
                  ? "bg-[#1f6feb] text-white"
                  : "text-gray-400 hover:bg-[#21262d]"
              }`}
            >
              平日
            </button>

            <button
              onClick={() => setTab("saturdayHoliday")}
              className={`flex-1 py-3 text-sm font-medium transition ${
                tab === "saturdayHoliday"
                  ? "bg-[#1f6feb] text-white"
                  : "text-gray-400 hover:bg-[#21262d]"
              }`}
            >
              土休日
            </button>
          </div>

          {tab === "weekday" && (
            <TimetableCard
              title="平日"
              timetable={weekday}
              trainLocationAvailable={trainLocationAvailable}
            />
          )}

          {tab === "saturdayHoliday" && (
            <TimetableCard
              title="土休日"
              timetable={saturdayHoliday}
              trainLocationAvailable={trainLocationAvailable}
            />
          )}
        </div>

        {/* Desktop */}
        <div className="hidden gap-6 lg:grid lg:grid-cols-2">
          <TimetableCard
            title="平日"
            timetable={weekday}
            trainLocationAvailable={trainLocationAvailable}
          />
          <TimetableCard
            title="土休日"
            timetable={saturdayHoliday}
            trainLocationAvailable={trainLocationAvailable}
          />
        </div>
      </>
    );
  }

  return (
    <>
      {/* Mobile */}
      <div className="lg:hidden">
        <div className="mb-5 flex overflow-hidden rounded-xl border border-[#30363d] bg-[#161b22]">
          <button
            onClick={() => setTab("weekday")}
            className={`flex-1 py-3 text-sm font-medium transition ${
              tab === "weekday"
                ? "bg-[#1f6feb] text-white"
                : "text-gray-400 hover:bg-[#21262d]"
            }`}
          >
            平日
          </button>

          <button
            onClick={() => setTab("saturday")}
            className={`flex-1 py-3 text-sm font-medium transition ${
              tab === "saturday"
                ? "bg-[#1f6feb] text-white"
                : "text-gray-400 hover:bg-[#21262d]"
            }`}
          >
            土曜
          </button>

          <button
            onClick={() => setTab("holiday")}
            className={`flex-1 py-3 text-sm font-medium transition ${
              tab === "holiday"
                ? "bg-[#1f6feb] text-white"
                : "text-gray-400 hover:bg-[#21262d]"
            }`}
          >
            休日
          </button>
        </div>

        {tab === "weekday" && (
          <TimetableCard
            title="平日"
            timetable={weekday}
            trainLocationAvailable={trainLocationAvailable}
          />
        )}

        {tab === "saturday" && (
          <TimetableCard
            title="土曜"
            timetable={saturday}
            trainLocationAvailable={trainLocationAvailable}
          />
        )}

        {tab === "holiday" && (
          <TimetableCard
            title="休日"
            timetable={holiday}
            trainLocationAvailable={trainLocationAvailable}
          />
        )}
      </div>

      {/* Desktop */}
      <div className="hidden gap-6 lg:grid lg:grid-cols-3">
        <TimetableCard
          title="平日"
          timetable={weekday}
          trainLocationAvailable={trainLocationAvailable}
        />
        <TimetableCard
          title="土曜"
          timetable={saturday}
          trainLocationAvailable={trainLocationAvailable}
        />
        <TimetableCard
          title="休日"
          timetable={holiday}
          trainLocationAvailable={trainLocationAvailable}
        />
      </div>
    </>
  );
}
