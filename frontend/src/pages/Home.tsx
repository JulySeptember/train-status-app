import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { arrayMove } from "@dnd-kit/sortable";
import {
  ChevronRight,
  CircleCheck,
  Route,
  RotateCcw,
  SendHorizontal,
  Sparkles,
  TriangleAlert,
} from "lucide-react";

import { api } from "@/api";
import { type Railway, type TrainStatus } from "@/types";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import SortableList from "@/components/SortableList";
import RailwayBadge from "@/components/RailwayBadge";
import JourneyForm from "@/components/JourneyForm";
import PageTitle from "@/components/PageTitle";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";

import { applyOrder, sortByOrder, useRailwayOrder } from "@/lib/railwayOrder";
import { useRailways } from "@/lib/railways";
import { EXAMPLES, MAX_INPUT_CHARS, type ChatLocationState } from "@/lib/chat";
import { cn } from "@/lib/utils";

// 遅れが無いときの運行情報の文章。全角・半角の数字のどちらでも配信される
function isNormal(status: string) {
  return (
    status.includes("現在、15分以上の遅延はありません") ||
    status.includes("現在、１５分以上の遅延はありません")
  );
}

// 質問を「AI に聞く」の画面に渡して開く。送るのは向こうの画面で行う
function AskForm() {
  const navigate = useNavigate();
  const [input, setInput] = useState("");

  const ask = (text: string) => {
    const question = text.trim();
    if (question) {
      navigate("/chat", { state: { question } satisfies ChatLocationState });
    }
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        ask(input);
      }}
      className="space-y-3"
    >
      <div className="flex items-end gap-2">
        <Textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            // 日本語の変換を確定する Enter では送らない
            if (
              e.key === "Enter" &&
              !e.shiftKey &&
              !e.nativeEvent.isComposing
            ) {
              e.preventDefault();
              ask(input);
            }
          }}
          maxLength={MAX_INPUT_CHARS}
          placeholder="例: 今春日にいる。浅草に行きたい"
          aria-label="AI への質問"
          className="max-h-40 min-h-12 text-base"
        />

        <Button
          type="submit"
          size="icon-lg"
          disabled={!input.trim()}
          aria-label="送信"
        >
          <SendHorizontal />
        </Button>
      </div>

      <div className="flex flex-wrap gap-2">
        {EXAMPLES.map((example) => (
          <button
            key={example}
            type="button"
            onClick={() => ask(example)}
            className="rounded-full border px-3 py-1 text-left text-sm text-foreground/80 transition hover:bg-muted hover:text-foreground"
          >
            {example}
          </button>
        ))}
      </div>

      <p className="text-xs text-muted-foreground">
        経路・次の電車・運行状況を、話し言葉で聞けます。遅延や運転見合わせを確かめてから経路を提案します。
      </p>
    </form>
  );
}

function SearchPanel() {
  const navigate = useNavigate();

  return (
    <Card className="[--card-spacing:--spacing(5)]">
      <CardContent>
        <Tabs defaultValue="journey" className="gap-4">
          <TabsList className="h-10! w-full sm:w-fit">
            <TabsTrigger value="journey" className="px-4 text-base">
              <Route />
              経路検索
            </TabsTrigger>
            <TabsTrigger value="ai" className="px-4 text-base">
              <Sparkles />
              AI に聞く
            </TabsTrigger>
          </TabsList>

          <TabsContent value="journey">
            <JourneyForm
              onSearch={(params) => navigate(`/journeys?${params}`)}
            />
          </TabsContent>

          <TabsContent value="ai">
            <AskForm />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}

function StatusCard({
  railway,
  status,
}: {
  railway: Railway;
  status?: TrainStatus;
}) {
  const normal = status ? isNormal(status.status) : true;

  return (
    <Link
      to={`/routes/${encodeURIComponent(railway.id)}`}
      className={cn(
        "flex h-full items-center gap-3 rounded-xl border bg-card px-4 py-3 transition hover:bg-muted",
        !normal && "border-destructive/60 bg-destructive/10",
      )}
    >
      <RailwayBadge railway={railway} className="size-8 text-sm" />

      <div className="min-w-0 flex-1">
        <p className="font-semibold">{railway.name}</p>

        {!status ? (
          <p className="text-sm text-muted-foreground">運行情報なし</p>
        ) : normal ? (
          <p className="flex items-center gap-1 text-sm text-brand">
            <CircleCheck size={14} />
            平常運転
          </p>
        ) : (
          <p className="flex items-start gap-1 text-sm text-destructive-foreground">
            <TriangleAlert size={14} className="mt-0.5 shrink-0" />
            {status.status}
          </p>
        )}
      </div>

      <ChevronRight size={18} className="shrink-0 text-muted-foreground" />
    </Link>
  );
}

function StatusSection() {
  const status = useQuery({
    queryKey: ["status"],
    queryFn: api.getStatus,
  });
  const railways = useRailways();
  const railwayOrder = useRailwayOrder();

  if (status.isPending || railways.isPending) {
    return <Loading />;
  }

  if (status.error || railways.error) {
    return <Error />;
  }

  const order = applyOrder(
    railways.data.map((r) => r.id),
    railwayOrder.order,
  );
  const items = sortByOrder(railways.data, (r) => r.id, order);
  const statusOf = (id: string) => status.data.find((s) => s.railwayId === id);

  const delayed = railways.data.filter((r) => {
    const s = statusOf(r.id);
    return s && !isNormal(s.status);
  });

  const move = (activeId: string, overId: string) => {
    railwayOrder.setOrder(
      arrayMove(order, order.indexOf(activeId), order.indexOf(overId)),
    );
  };

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <h2 className="text-2xl font-bold">運行情報</h2>

          <p
            className={cn(
              "mt-1 text-sm",
              delayed.length > 0
                ? "text-destructive-foreground"
                : "text-muted-foreground",
            )}
          >
            {delayed.length > 0
              ? `${delayed.map((r) => r.name).join("・")}で遅れが出ています。`
              : "すべての路線で、15分以上の遅れはありません。"}
          </p>
        </div>

        {railwayOrder.order && (
          <Button variant="outline" size="sm" onClick={railwayOrder.reset}>
            <RotateCcw />
            並び順を戻す
          </Button>
        )}
      </div>

      <SortableList
        items={items}
        getId={(r) => r.id}
        getLabel={(r) => r.name}
        onMove={move}
        layout="grid"
        className="grid gap-3 md:grid-cols-2"
        renderItem={(r) => <StatusCard railway={r} status={statusOf(r.id)} />}
      />

      <p className="text-xs text-muted-foreground">
        左のつまみをドラッグすると、路線を並び替えられます。路線を選ぶと駅の一覧を開きます。
      </p>
    </section>
  );
}

export default function Home() {
  return (
    <div className="space-y-10">
      <PageTitle />
      <SearchPanel />
      <StatusSection />
    </div>
  );
}
