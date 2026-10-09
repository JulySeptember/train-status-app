import { Link, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Sparkles } from "lucide-react";

import { api } from "@/api";

import Loading from "@/components/Loading";
import JourneyForm, { readJourneyQuery } from "@/components/JourneyForm";
import JourneyList from "@/components/JourneyList";
import RealtimeNotice from "@/components/RealtimeNotice";
import PageTitle from "@/components/PageTitle";

import { Card, CardContent } from "@/components/ui/card";

function Message({ title, children }: { title: string; children: string }) {
  return (
    <div className="rounded-xl border p-6 text-center">
      <p className="text-lg font-medium">{title}</p>

      <p className="mt-2 text-sm text-muted-foreground">{children}</p>
    </div>
  );
}

export default function Journey() {
  const [params, setParams] = useSearchParams();
  const query = readJourneyQuery(params);

  const journeys = useQuery({
    queryKey: ["journeys", query],
    queryFn: () => api.searchJourneys(query!),
    enabled: query !== null,
  });

  // 徒歩の区間の案内に、指定した駅の名前を使う（検索の入力欄と同じキャッシュ）
  const stations = useQuery({
    queryKey: ["stations"],
    queryFn: api.getAllStations,
  });
  const stationName = (id: string) =>
    stations.data?.find((s) => s.id === id)?.name;

  return (
    <div className="space-y-8">
      <PageTitle title="経路検索" />
      <h1 className="text-3xl font-bold">経路検索</h1>

      <Card>
        <CardContent>
          {/* URL が変わったら（ホームからの検索・戻る）入力欄を作り直して条件を合わせる */}
          <JourneyForm
            key={params.toString()}
            initial={query}
            onSearch={setParams}
          />
        </CardContent>
      </Card>

      {query && journeys.isPending && <Loading />}

      {query && journeys.error && (
        <Message title="経路を検索できませんでした。">
          駅や時刻の指定を確かめて、もう一度検索してください。
        </Message>
      )}

      {query && journeys.data && (
        <RealtimeNotice
          data={journeys.data}
          realtime={query.realtime ?? true}
        />
      )}

      {query && journeys.data?.journeys.length === 0 && (
        <div className="space-y-3">
          <Message title="経路が見つかりませんでした。">
            終電を過ぎているか、乗り換え3回までではたどり着けない可能性があります。
          </Message>

          <p className="text-center text-sm text-muted-foreground">
            <Link
              to="/chat"
              className="inline-flex items-center gap-1 text-brand hover:underline"
            >
              <Sparkles size={14} />
              AI 乗換相談
            </Link>
            なら、話し言葉で行き方を調べられます。
          </p>
        </div>
      )}

      {query && journeys.data && journeys.data.journeys.length > 0 && (
        <JourneyList
          journeys={journeys.data.journeys}
          origin={stationName(query.from)}
          destination={stationName(query.to)}
        />
      )}
    </div>
  );
}
