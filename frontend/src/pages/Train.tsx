import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";

import { api } from "@/api";
import { type TrainLocation as TrainLocationType } from "@/types";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import TrainLocation, { TrainNotRunning } from "@/components/TrainLocation";
import PageTitle from "@/components/PageTitle";

import { nowServiceMinutes, timeToServiceMinutes } from "@/lib/time";

// 「運行を終えました」と出したあとも取り直し続ける時間（分）。
// 判定は時刻表の時刻だけなので、遅れて走っている列車が配信から一時的に抜けたときにも出る
const FINISHED_RECHECK_MINUTES = 30;

// 列車位置を取り直す間隔（ミリ秒）。false なら取り直さない
function refetchInterval(data?: TrainLocationType): number | false {
  if (!data) {
    return false;
  }

  if (data.available || data.notRunning === "noData") {
    return 15_000;
  }

  const scheduled = data.scheduledTime
    ? timeToServiceMinutes(data.scheduledTime)
    : null;

  if (scheduled === null) {
    return false;
  }

  const now = nowServiceMinutes();

  // 出発までが長い列車は、開いたままにしても API を呼び続けないよう間隔を延ばす
  if (data.notRunning === "beforeDeparture") {
    const untilDeparture = scheduled - now;

    if (untilDeparture > 30) {
      return 5 * 60_000;
    }

    if (untilDeparture > 5) {
      return 60_000;
    }

    return 15_000;
  }

  if (
    data.notRunning === "finished" &&
    now - scheduled <= FINISHED_RECHECK_MINUTES
  ) {
    return 60_000;
  }

  return false;
}

export default function Train() {
  const { trainId = "" } = useParams();

  const { data, isPending, error } = useQuery({
    queryKey: ["train", trainId],
    queryFn: () => api.getTrain(trainId),

    staleTime: 15_000,

    // 走っている列車は位置を更新し、まだ出発していない列車は出発したら位置を出すために取り直す
    refetchInterval: (query) => refetchInterval(query.state.data),
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
