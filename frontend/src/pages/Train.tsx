import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import TrainLocation from "@/components/TrainLocation";

export default function Train() {
  const { trainNumber = "" } = useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ["train", trainNumber],
    queryFn: () => api.getTrain(trainNumber),

    staleTime: 15_000,

    refetchInterval: (query) => {
      const data = query.state.data;

      if (data?.available) {
        return 15000;
      }

      return false;
    },
  });

  if (isPending) {
    return <Loading />;
  }

  if (error || !data) {
    return <Error />;
  }

  if (!data.available) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-bold">列車情報</h1>

        <p>{data.message}</p>
      </div>
    );
  }

  return <TrainLocation train={data} />;
}
