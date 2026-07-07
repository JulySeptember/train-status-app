import { useState } from "react";
import { Link } from "react-router-dom";
import { Clock3, ArrowRight } from "lucide-react";

import { type DirectionTimetable } from "@/types";

type Props = {
  weekday?: DirectionTimetable;
  saturday?: DirectionTimetable;
  holiday?: DirectionTimetable;
  saturdayHoliday?: DirectionTimetable;
};

function TimetableCard({
  title,
  timetable,
}: {
  title: string;
  timetable?: DirectionTimetable;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-[#30363d] bg-[#0d1117]">
      <div className="flex items-center gap-2 border-b border-[#30363d] px-5 py-4">
        <Clock3 size={18} className="text-[#2ea043]" />

        <h3 className="font-semibold text-white">{title}</h3>
      </div>

      <div className="divide-y divide-[#30363d]">
        {timetable?.timetables?.length ? (
          timetable.timetables.map((train) => (
            <Link
              key={`${train.time}-${train.trainNumber}`}
              to={`/trains/${train.trainNumber}`}
              className="flex items-center justify-between px-5 py-4 transition hover:bg-[#161b22]"
            >
              <div>
                <p className="text-2xl font-bold text-white">{train.time}</p>

                <p className="mt-1 text-sm text-gray-400">
                  列車番号 {train.trainNumber}
                </p>
              </div>

              <ArrowRight size={18} className="text-gray-500" />
            </Link>
          ))
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
            <TimetableCard title="平日" timetable={weekday} />
          )}

          {tab === "saturdayHoliday" && (
            <TimetableCard title="土休日" timetable={saturdayHoliday} />
          )}
        </div>

        {/* Desktop */}
        <div className="hidden gap-6 lg:grid lg:grid-cols-2">
          <TimetableCard title="平日" timetable={weekday} />
          <TimetableCard title="土休日" timetable={saturdayHoliday} />
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
          <TimetableCard title="平日" timetable={weekday} />
        )}

        {tab === "saturday" && (
          <TimetableCard title="土曜" timetable={saturday} />
        )}

        {tab === "holiday" && (
          <TimetableCard title="休日" timetable={holiday} />
        )}
      </div>

      {/* Desktop */}
      <div className="hidden gap-6 lg:grid lg:grid-cols-3">
        <TimetableCard title="平日" timetable={weekday} />
        <TimetableCard title="土曜" timetable={saturday} />
        <TimetableCard title="休日" timetable={holiday} />
      </div>
    </>
  );
}
