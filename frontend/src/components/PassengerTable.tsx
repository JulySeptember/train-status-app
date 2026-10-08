import { Users } from "lucide-react";

import { type Passenger } from "@/types";

type Props = {
  passengers: Passenger[];
};

export default function PassengerTable({ passengers }: Props) {
  if (!passengers || passengers.length === 0) {
    return (
      <div className="rounded-xl border border-border bg-background py-10 text-center text-muted-foreground">
        乗降者データがありません
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-xl border border-border bg-background">
      <div className="flex items-center gap-3 border-b border-border px-6 py-4">
        <Users className="text-brand" size={18} />

        <div>
          <h3 className="font-semibold text-foreground">年度別乗降者数</h3>

          <p className="text-sm text-muted-foreground">
            東京都交通局オープンデータ
          </p>
        </div>
      </div>

      <table className="w-full">
        <thead className="bg-card">
          <tr className="border-b border-border">
            <th className="px-6 py-3 text-left text-sm font-medium text-muted-foreground">
              年度
            </th>

            <th className="px-6 py-3 text-right text-sm font-medium text-muted-foreground">
              乗降者数
            </th>
          </tr>
        </thead>

        <tbody>
          {passengers.map((row) => (
            <tr
              key={row.year}
              className="border-b border-border transition hover:bg-card"
            >
              <td className="px-6 py-4 font-medium text-foreground">
                {row.year}
              </td>

              <td className="px-6 py-4 text-right">
                <span className="font-semibold text-foreground">
                  {row.count.toLocaleString()}
                </span>

                <span className="ml-1 text-muted-foreground">人</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
