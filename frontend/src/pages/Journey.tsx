import { Link, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Sparkles } from "lucide-react";

import { api, isClientError } from "@/api";

import Loading from "@/components/Loading";
import JourneyForm, { readJourneyQuery } from "@/components/JourneyForm";
import JourneyList from "@/components/JourneyList";
import RealtimeNotice from "@/components/RealtimeNotice";
import PageTitle from "@/components/PageTitle";
import FavoriteButton from "@/components/FavoriteButton";
import SegmentedControl from "@/components/SegmentedControl";

import { journeyFavoriteId } from "@/lib/favorites";
import {
  type JourneySort,
  SORT_PARAM,
  journeySortOptions,
  readJourneySort,
  sortJourneys,
} from "@/lib/journeySort";

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
  const sort = readJourneySort(params);

  // 並び順を変えても検索はし直さない（queryKey に入らない）。戻るで並び順を1つずつたどらないよう、履歴は置き換える
  const setSort = (value: JourneySort) => {
    const next = new URLSearchParams(params);
    if (value === "time") {
      next.delete(SORT_PARAM);
    } else {
      next.set(SORT_PARAM, value);
    }
    setParams(next, { replace: true });
  };

  // 検索し直しても、選んでいた並び順は保つ
  const search = (next: URLSearchParams) => {
    if (sort !== "time") {
      next.set(SORT_PARAM, sort);
    }
    setParams(next);
  };

  // 入力欄を作り直す鍵。並び順だけが変わったときは作り直さない（入力中の条件を消さない）
  const formKey = (() => {
    const p = new URLSearchParams(params);
    p.delete(SORT_PARAM);
    return p.toString();
  })();

  const journeys = useQuery({
    queryKey: ["journeys", query],
    queryFn: ({ signal }) => api.searchJourneys(query!, { signal }),
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
          <JourneyForm key={formKey} initial={query} onSearch={search} />
        </CardContent>
      </Card>

      {query && journeys.isPending && <Loading />}

      {/* 4xx は指定の誤り、5xx や通信の失敗はサーバー側の問題なので、指定を見直すよう案内しない */}
      {query && journeys.error && (
        <Message title="経路を検索できませんでした。">
          {isClientError(journeys.error)
            ? "駅や時刻の指定を確かめて、もう一度検索してください。"
            : "通信できなかったか、サーバーでエラーが発生しました。時間をおいて、もう一度検索してください。"}
        </Message>
      )}

      {query && journeys.data && (
        <div className="flex flex-wrap items-center gap-3">
          {/* 経路が1本なら並べ替えても変わらないので出さない */}
          {journeys.data.journeys.length > 1 && (
            <SegmentedControl
              options={journeySortOptions(query.arriveBy !== undefined)}
              value={sort}
              onChange={setSort}
              label="並び順"
              optionClassName="px-2"
            />
          )}

          <div className="ml-auto">
            <FavoriteButton
              kind="journeys"
              id={journeyFavoriteId(query.from, query.to)}
              name={`${stationName(query.from) ?? "出発駅"}から${stationName(query.to) ?? "到着駅"}までの経路`}
              label
            />
          </div>
        </div>
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
          journeys={sortJourneys(journeys.data.journeys, sort)}
          origin={stationName(query.from)}
          destination={stationName(query.to)}
        />
      )}
    </div>
  );
}
