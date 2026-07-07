import { Users } from "lucide-react";

import { type Passenger } from "@/types";

type Props = {
  passengers: Passenger[];
};

export default function PassengerTable({ passengers }: Props) {
  if (!passengers || passengers.length === 0) {
    return (
      <div className="rounded-xl border border-[#30363d] bg-[#0d1117] py-10 text-center text-gray-500">
        乗降者データがありません
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border border-[#30363d] bg-[#0d1117]">
      <div className="flex items-center gap-3 border-b border-[#30363d] px-6 py-4">
        <Users className="text-[#58a6ff]" size={18} />

        <div>
          <h3 className="font-semibold text-white">年度別乗降者数</h3>

          <p className="text-sm text-gray-400">東京都交通局オープンデータ</p>
        </div>
      </div>

      <table className="w-full">
        <thead className="bg-[#161b22]">
          <tr className="border-b border-[#30363d]">
            <th className="px-6 py-3 text-left text-sm font-medium text-gray-400">
              年度
            </th>

            <th className="px-6 py-3 text-right text-sm font-medium text-gray-400">
              乗降者数
            </th>
          </tr>
        </thead>

        <tbody>
          {passengers.map((row) => (
            <tr
              key={row.year}
              className="border-b border-[#30363d] transition hover:bg-[#161b22]"
            >
              <td className="px-6 py-4 font-medium text-white">{row.year}</td>

              <td className="px-6 py-4 text-right">
                <span className="font-semibold text-white">
                  {row.count.toLocaleString()}
                </span>

                <span className="ml-1 text-gray-400">人</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
