import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { arrayMove } from "@dnd-kit/sortable";
import {
  ArrowUpDown,
  Check,
  ChevronRight,
  CircleCheck,
  Info,
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
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";

import {
  applyOrder,
  sortByOrder,
  useOperatorOrder,
  useRailwayOrder,
} from "@/lib/railwayOrder";
import { groupByOperator, useRailways } from "@/lib/railways";
import { EXAMPLES, MAX_INPUT_CHARS, type ChatLocationState } from "@/lib/chat";
import { cn } from "@/lib/utils";

// 運行情報の文章に含まれていたら平常とみなす言葉（バックエンドの service/assistant.go の normalWords と揃える）。
// 「１５分以上の遅延はありません」のように全角の数字でも配信されるので、数字を含めずに判定する
const NORMAL_WORDS = ["遅延はありません", "平常"];

function isNormal(status: string) {
  return NORMAL_WORDS.some((w) => status.includes(w));
}

// 質問を「AI 乗換相談」の画面に渡して開く。送るのは向こうの画面で行う
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
          placeholder="行きたい場所や、知りたいことを入力"
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

      {/* 送るとすぐに AI に渡るので、「AI 乗換相談」の画面と同じ注意をここでも出す */}
      <p className="flex items-start gap-1.5 text-xs text-muted-foreground">
        <Info size={14} className="mt-px shrink-0" />
        入力内容は AI
        の提供元（Google）に送信され、サービス改善に利用されることがあります。個人情報は入力しないでください。
      </p>
    </form>
  );
}

function SearchPanel() {
  const navigate = useNavigate();
  // 選んだタブは URL に残す（経路検索の結果から戻ったとき、経路検索のタブを開いたままにする）
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") === "journey" ? "journey" : "ai";

  return (
    <Card className="[--card-spacing:--spacing(5)]">
      <CardContent>
        <Tabs
          value={tab}
          onValueChange={(value) =>
            setParams(value === "journey" ? { tab: "journey" } : {}, {
              replace: true,
            })
          }
          className="gap-4"
        >
          <TabsList className="h-10! w-full sm:w-fit">
            <TabsTrigger value="ai" className="px-4 text-base">
              <Sparkles />
              AI 乗換相談
            </TabsTrigger>
            <TabsTrigger value="journey" className="px-4 text-base">
              <Route />
              経路検索
            </TabsTrigger>
          </TabsList>

          <TabsContent value="ai">
            <AskForm />
          </TabsContent>

          <TabsContent value="journey">
            <JourneyForm
              onSearch={(params) => navigate(`/journeys?${params}`)}
            />
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
  status: TrainStatus;
}) {
  const normal = status.unavailable || isNormal(status.status);

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

        {status.unavailable ? (
          <p className="text-sm text-muted-foreground">{status.status}</p>
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

// 事業者ごとの見出し。閉じていても遅れの有無が分かるようにする
function OperatorSummary({
  name,
  statuses,
}: {
  name: string;
  statuses: TrainStatus[];
}) {
  const delayed = statuses.filter(
    (s) => !s.unavailable && !isNormal(s.status),
  ).length;

  let summary;
  if (delayed > 0) {
    summary = (
      <span className="flex items-center gap-1 text-destructive-foreground">
        <TriangleAlert size={14} className="shrink-0" />
        {delayed}路線で遅れ
      </span>
    );
  } else if (statuses.some((s) => s.unavailable)) {
    summary = (
      <span className="text-muted-foreground">取得できませんでした</span>
    );
  } else {
    summary = (
      <span className="flex items-center gap-1 text-brand">
        <CircleCheck size={14} className="shrink-0" />
        平常運転
      </span>
    );
  }

  return (
    <span className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
      <span className="text-lg font-semibold">{name}</span>
      <span className="text-sm font-normal">{summary}</span>
      <span className="text-xs font-normal text-muted-foreground">
        {statuses.length}路線
      </span>
    </span>
  );
}

function StatusSection() {
  const status = useQuery({
    queryKey: ["status"],
    queryFn: api.getStatus,
  });
  const railways = useRailways();
  const railwayOrder = useRailwayOrder();
  const operatorOrder = useOperatorOrder();

  // 開いている事業者。開け閉めするまでは、遅れのある事業者だけを開く
  const [opened, setOpened] = useState<string[] | null>(null);
  // 事業者の並び替えは、アコーディオンの代わりに事業者の名前だけを並べて行う。
  // アコーディオンのまま動かすと、開いた事業者の高さで矢印キーやドラッグの位置がずれ、路線のつまみとも入れ子になる
  const [sortingOperators, setSortingOperators] = useState(false);

  if (status.isPending || railways.isPending) {
    return <Loading />;
  }

  if (status.error || railways.error) {
    return <Error />;
  }

  // 小田急など運行情報を配信していない路線は出さない。取得に失敗した路線（unavailable）は配信元から返るので残る
  const statuses = new Map(status.data.map((s) => [s.railwayId, s]));
  const items = railways.data.flatMap((railway) => {
    const s = statuses.get(railway.id);
    return s ? [{ railway, status: s }] : [];
  });

  // 並び順は表示しない路線も含めた全路線で持つ。のちに配信が始まった路線も、並べた位置に出る
  const order = applyOrder(
    railways.data.map((r) => r.id),
    railwayOrder.order,
  );
  const sorted = sortByOrder(items, (i) => i.railway.id, order);

  type Item = (typeof items)[number];
  const isDelayed = (i: Item) =>
    !i.status.unavailable && !isNormal(i.status.status);
  const delayed = sorted.filter(isDelayed);

  // 路線は事業者の中で並び替え、事業者のまとまりはまとまりごと並び替える
  // 事業者の順も、路線と同じく表示しない事業者を含めて持つ
  const groupOrder = applyOrder(
    railways.data.map((r) => r.operator ?? ""),
    operatorOrder.order,
  );
  const groups = sortByOrder(
    groupByOperator(sorted, (i) => i.railway),
    (g) => g.operator,
    groupOrder,
  );

  const moveRailway = (activeId: string, overId: string) => {
    railwayOrder.setOrder(
      arrayMove(order, order.indexOf(activeId), order.indexOf(overId)),
    );
  };

  const moveOperator = (activeId: string, overId: string) => {
    operatorOrder.setOrder(
      arrayMove(
        groupOrder,
        groupOrder.indexOf(activeId),
        groupOrder.indexOf(overId),
      ),
    );
  };

  const list = (items: Item[]) => (
    <SortableList
      items={items}
      getId={(i) => i.railway.id}
      getLabel={(i) => i.railway.name}
      onMove={moveRailway}
      layout="grid"
      className="grid gap-3 md:grid-cols-2"
      renderItem={(i) => <StatusCard railway={i.railway} status={i.status} />}
    />
  );

  const open =
    opened ??
    groups.filter((g) => g.items.some(isDelayed)).map((g) => g.operator);

  const reordered = railwayOrder.order || operatorOrder.order;
  // 事業者が1つ以下になったら（再取得で他社が消えたときなど）並び替えのモードを抜ける
  const sorting = sortingOperators && groups.length > 1;

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
              ? `${delayed.map((i) => i.railway.name).join("・")}で遅れが出ています。`
              : items.length > 0 &&
                "すべての路線で、15分以上の遅れはありません。"}
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          {groups.length > 1 && (
            <Button
              variant={sorting ? "default" : "outline"}
              size="sm"
              aria-pressed={sorting}
              onClick={() => setSortingOperators(!sorting)}
            >
              {sorting ? <Check /> : <ArrowUpDown />}
              {sorting ? "並び替えを終える" : "事業者を並び替える"}
            </Button>
          )}

          {reordered && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                railwayOrder.reset();
                operatorOrder.reset();
              }}
            >
              <RotateCcw />
              並び順を戻す
            </Button>
          )}
        </div>
      </div>

      {/* 事業者が1つ（都営だけ）なら、まとめずに並べる */}
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          運行情報を配信している路線がありません。
        </p>
      ) : groups.length <= 1 ? (
        list(sorted)
      ) : sorting ? (
        <SortableList
          items={groups}
          getId={(g) => g.operator}
          getLabel={(g) => g.name}
          onMove={moveOperator}
          className="flex flex-col gap-3"
          renderItem={(g) => (
            <div className="rounded-xl border bg-card px-4 py-3">
              <OperatorSummary
                name={g.name}
                statuses={g.items.map((i) => i.status)}
              />
            </div>
          )}
        />
      ) : (
        <Accordion
          multiple
          value={open}
          onValueChange={(value) => setOpened(value as string[])}
          className="gap-3"
        >
          {groups.map((g) => (
            <AccordionItem
              key={g.operator}
              value={g.operator}
              className="rounded-xl border bg-card px-4"
            >
              <AccordionTrigger className="items-center py-3 hover:no-underline">
                <OperatorSummary
                  name={g.name}
                  statuses={g.items.map((i) => i.status)}
                />
              </AccordionTrigger>

              <AccordionContent className="pb-4">
                {list(g.items)}
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      )}

      <p className="text-xs text-muted-foreground">
        {sorting
          ? "左のつまみをドラッグすると、事業者を並び替えられます。"
          : "左のつまみをドラッグすると、路線を並び替えられます。路線を選ぶと駅の一覧を開きます。"}
        運行情報を配信していない路線は、「駅・時刻表」から開けます。
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
