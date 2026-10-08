import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import TrainLocation, { TrainNotRunning } from "@/components/TrainLocation";
import PageTitle from "@/components/PageTitle";

export default function Train() {
  const { trainId = "" } = useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ["train", trainId],
    queryFn: () => api.getTrain(trainId),

    staleTime: 15_000,

    // 走っている列車と、まだ出発していない列車は、出発したら位置を出すために取り直す
    refetchInterval: (query) => {
      const data = query.state.data;

      if (
        data?.available ||
        data?.notRunning === "beforeDeparture" ||
        data?.notRunning === "noData"
      ) {
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

  const title = `${data.railway ? `${data.railway} ` : ""}${data.trainNumber}`;

  if (!data.available) {
    return (
      <>
        <PageTitle title={`${title} の運行状況`} />
        <TrainNotRunning train={data} />
      </>
    );
  }

  return (
    <>
      <PageTitle title={`${title} の位置`} />
      <TrainLocation train={data} />
    </>
  );
}
