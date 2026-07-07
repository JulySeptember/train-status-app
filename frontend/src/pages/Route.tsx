import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";

import { Card, CardContent } from "@/components/ui/card";

export default function Route() {
  const { routeId = "" } = useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ["stations", routeId],
    queryFn: () => api.getStations(routeId),
  });

  if (isPending) return <Loading />;

  if (error) return <Error />;

  return (
    <div className="space-y-4">
      <h1 className="text-3xl font-bold">駅一覧</h1>

      {data?.map((station) => (
        <Link key={station.id} to={`/stations/${station.id}`}>
          <Card className="hover:bg-accent">
            <CardContent className="py-4">{station.name}</CardContent>
          </Card>
        </Link>
      ))}
    </div>
  );
}
