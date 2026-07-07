import { MapPin, Train, Timer } from "lucide-react";

import { type TrainLocation as TrainLocationType } from "@/types";

type Props = {
  train: TrainLocationType;
};

export default function TrainLocation({ train }: Props) {
  return (
    <section className="overflow-hidden rounded-xl border border-[#30363d] bg-[#161b22]">
      <div className="border-b border-[#30363d] px-6 py-5">
        <div className="flex items-center gap-3">
          <Train className="text-[#58a6ff]" size={20} />

          <div>
            <h2 className="text-lg font-semibold text-white">列車現在位置</h2>

            <p className="text-sm text-gray-400">リアルタイム運行情報</p>
          </div>
        </div>
      </div>

      <div className="space-y-6 p-6">
        <div className="rounded-xl border border-[#30363d] bg-[#0d1117] p-5">
          <div className="mb-2 text-xs uppercase tracking-wider text-gray-500">
            路線
          </div>

          <p className="text-xl font-semibold text-white">{train.railway}</p>
        </div>

        <div className="rounded-xl border border-[#30363d] bg-[#0d1117] p-6">
          <div className="mb-6 flex items-center justify-between text-sm text-gray-400">
            <span>{train.fromStation}</span>

            <span>{train.toStation}</span>
          </div>

          <div className="relative">
            <div className="h-1 rounded-full bg-[#30363d]" />

            <div className="absolute left-1/2 top-1/2 h-5 w-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-4 border-[#161b22] bg-[#58a6ff]" />
          </div>

          <div className="mt-5 flex items-center justify-center gap-2 text-[#58a6ff]">
            <MapPin size={18} />

            <span className="font-medium">
              {train.fromStation} → {train.toStation}
            </span>
          </div>
        </div>

        <div className="rounded-xl border border-[#30363d] bg-[#0d1117] p-5">
          <div className="flex items-center gap-2 text-gray-400">
            <Timer size={18} />

            <span>遅延時間</span>
          </div>

          <p className="mt-3 text-3xl font-bold text-white">
            {train.delay}
            <span className="ml-2 text-lg font-normal text-gray-400">秒</span>
          </p>
        </div>
      </div>
    </section>
  );
}
