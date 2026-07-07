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
      <div className="rounded-xl border border-[#30363d] bg-[#161b22] p-8">
        <h1 className="mb-4 text-2xl font-bold text-white">列車情報</h1>

        <p className="mb-6 text-gray-300">{data.message}</p>

        <div className="rounded-lg border border-[#30363d] bg-[#0d1117] p-5">
          <h2 className="mb-3 text-lg font-semibold text-white">
            考えられる理由
          </h2>

          <ul className="list-disc space-y-2 pl-5 text-sm text-gray-400">
            <li>運行前または運行終了の列車です。</li>
            <li>現在位置情報が配信されていません。</li>
            <li>データ提供元で一時的に取得できない状態です。</li>
          </ul>
        </div>
      </div>
    );
  }
  return <TrainLocation train={data} />;
}
