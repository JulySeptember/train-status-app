import { useState } from "react";
import { Link } from "react-router-dom";
import { Clock3, ArrowRight } from "lucide-react";

import { type DirectionTimetable } from "@/types";

type Props = {
  weekday?: DirectionTimetable;
  saturday?: DirectionTimetable;
  holiday?: DirectionTimetable;
  saturdayHoliday?: DirectionTimetable;
  trainLocationAvailable: boolean;
};

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
          timetable.timetables.map((train) => {
            const content = (
              <div>
                <p className="text-2xl font-bold text-white">{train.time}</p>

                <p className="mt-1 text-sm text-gray-400">
                  列車番号 {train.trainNumber}
                </p>
              </div>
            );

            if (!linkable) {
              return (
                <div
                  key={`${train.time}-${train.trainId}`}
                  className="px-5 py-4"
                >
                  {content}
                </div>
              );
            }

            return (
              <Link
                key={`${train.time}-${train.trainId}`}
                to={`/trains/${encodeURIComponent(train.trainId)}`}
                className="flex items-center justify-between px-5 py-4 transition hover:bg-[#161b22]"
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
  const [tab, setTab] = useState<
    "weekday" | "saturday" | "holiday" | "saturdayHoliday"
  >("weekday");

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
