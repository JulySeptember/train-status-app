import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import FareSearch from "@/components/FareSearch";

export default function Fare() {
  const [fromId, setFromId] = useState("");
  const [toId, setToId] = useState("");

  const [search, setSearch] = useState<{
    from: string;
    to: string;
  } | null>(null);

  const stations = useQuery({
    queryKey: ["stations"],
    queryFn: api.getAllStations,
  });

  const fare = useQuery({
    queryKey: ["fare", search],
    queryFn: () => api.getFare(search!.from, search!.to),
    enabled: search !== null,
  });

  if (stations.isPending) {
    return <Loading />;
  }

  if (stations.error) {
    return <Error />;
  }

  return (
    <div className="space-y-8">
      <h1 className="text-3xl font-bold">運賃検索</h1>

      <FareSearch
        stations={stations.data ?? []}
        fromId={fromId}
        toId={toId}
        onFromChange={setFromId}
        onToChange={setToId}
        onSearch={() =>
          setSearch({
            from: fromId,
            to: toId,
          })
        }
        onReset={() => {
          setFromId("");
          setToId("");
          setSearch(null);
        }}
      />

      {search && fare.isPending && <Loading />}

      {search && fare.error && (
        <div className="rounded-lg border p-6 text-center">
          <p className="text-lg font-medium">
            この組み合わせの運賃は表示できません。
          </p>

          <p className="mt-2 text-sm text-muted-foreground">
            入力した駅の組み合わせでは運賃データがありません。
          </p>
        </div>
      )}

      {search && fare.data && (
        <div className="rounded-lg border p-6 space-y-3">
          <p className="text-xl">
            IC運賃：
            <strong>{fare.data.icFare}円</strong>
          </p>

          <p className="text-xl">
            きっぷ運賃：
            <strong>{fare.data.ticketFare}円</strong>
          </p>
        </div>
      )}
    </div>
  );
}
