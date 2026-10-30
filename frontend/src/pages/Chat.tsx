import { useEffect, useRef } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { useMutation, useMutationState } from "@tanstack/react-query";
import {
  CircleCheck,
  Info,
  LoaderCircle,
  RotateCcw,
  Route,
  SendHorizontal,
  TriangleAlert,
} from "lucide-react";

import { api, ChatError } from "@/api";
import {
  type ChatMessage,
  type ChatResponse,
  type ChatStep,
  type JourneySearch,
} from "@/types";

import JourneyList from "@/components/JourneyList";
import RealtimeNotice from "@/components/RealtimeNotice";
import PageTitle from "@/components/PageTitle";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  EXAMPLES,
  MAX_HISTORY_MESSAGES,
  MAX_INPUT_CHARS,
  type ChatLocationState,
} from "@/lib/chat";
import {
  addAnswer,
  addQuestion,
  dropQuestion,
  resetChat,
  setChatInput,
  useChatState,
  type ChatEntry,
} from "@/lib/chatStore";

// 上限に達したときなど、AI の代わりに通常の検索を案内するエラー
const FALLBACK_CODES = [
  "quota_exceeded",
  "rate_limited",
  "unavailable",
  "timeout",
];

// 回答を待っている質問の mutation。画面を離れて戻っても、待っていることを表示できるようにキーを付ける
const CHAT_MUTATION_KEY = ["chat"];

type ChatVariables = { history: ChatMessage[]; generation: number };

function Steps({ steps }: { steps: ChatStep[] }) {
  if (steps.length === 0) {
    return null;
  }

  return (
    <ul className="space-y-1 text-sm text-muted-foreground">
      {steps.map((step, i) => (
        <li key={i} className="flex items-start gap-1.5">
          <CircleCheck size={14} className="mt-0.5 shrink-0 text-brand" />
          {step.label}
        </li>
      ))}
    </ul>
  );
}

// AI が探した経路の出発駅・到着駅で、経路検索を開く（時刻などの条件を変えて探し直せるように）
function JourneySearchLink({ journeys }: { journeys: JourneySearch }) {
  const legs = journeys.journeys[0].legs;
  const params = new URLSearchParams({
    from: legs[0].from,
    to: legs[legs.length - 1].to,
  });

  return (
    <Link
      to={`/journeys?${params}`}
      className="inline-flex items-center gap-1 text-sm text-brand hover:underline"
    >
      <Route size={14} />
      経路検索で条件を変えて探す
    </Link>
  );
}

function AssistantMessage({
  entry,
}: {
  entry: Extract<ChatEntry, { role: "assistant" }>;
}) {
  const journeys = entry.journeys;

  return (
    <div className="space-y-4 rounded-xl border border-border bg-card p-4">
      <Steps steps={entry.steps} />

      <p className="leading-relaxed whitespace-pre-wrap text-foreground">
        {entry.text}
      </p>

      {journeys && journeys.journeys.length > 0 && (
        <div className="space-y-3 border-t border-border pt-4">
          <p className="text-xs text-muted-foreground">
            AI
            の回答は誤ることがあります。時刻は下の経路（時刻表のデータ）で確かめてください。
          </p>

          <RealtimeNotice data={journeys} realtime />

          <JourneyList journeys={journeys.journeys} />

          <JourneySearchLink journeys={journeys} />
        </div>
      )}
    </div>
  );
}

function ChatErrorMessage({ error }: { error: Error }) {
  const code = error instanceof ChatError ? error.code : "internal";
  const message =
    error instanceof ChatError
      ? error.message
      : "通信できませんでした。もう一度お試しください。";

  return (
    <Alert variant="destructive">
      <TriangleAlert />
      <AlertTitle>{message}</AlertTitle>

      {FALLBACK_CODES.includes(code) && (
        <AlertDescription>
          <p>
            <Link to="/journeys" className="underline">
              経路検索
            </Link>
            や
            <Link to="/" className="underline">
              運行情報
            </Link>
            は、AI を使わずにご利用いただけます。
          </p>
        </AlertDescription>
      )}
    </Alert>
  );
}

export default function Chat() {
  const { entries, generation, input, error } = useChatState();
  const bottomRef = useRef<HTMLDivElement>(null);

  // 回答の追加は、送るたびの callback ではなく useMutation の onSuccess・onError で行う。
  // 送るたびの callback は、回答を待つ間に別の画面へ移る（アンマウントする）と呼ばれず、回答が捨てられるため
  const chat = useMutation<ChatResponse, Error, ChatVariables>({
    mutationKey: CHAT_MUTATION_KEY,
    mutationFn: ({ history }) => api.chat(history),
    onSuccess: (res, { generation }) => {
      addAnswer(generation, {
        role: "assistant",
        text: res.reply,
        steps: res.steps,
        journeys: res.journeys,
      });
    },
    onError: (err, { history, generation }) => {
      // 答えられなかった質問は履歴から外し、入力欄に戻して送り直せるようにする
      dropQuestion(generation, history[history.length - 1].text, err);
    },
  });

  // 画面を離れて戻ったときも、この会話の質問の回答を待っていれば、待っている表示にし、続けて送らせない
  const pending = useMutationState({
    filters: { mutationKey: CHAT_MUTATION_KEY, status: "pending" },
    select: (m) => (m.state.variables as ChatVariables).generation,
  }).includes(generation);

  // 新しい発言・考え中の表示・エラーが出たら、そこまでスクロールする
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end", behavior: "smooth" });
  }, [entries.length, pending, error]);

  const send = (text: string) => {
    const question = text.trim();
    if (!question || pending) {
      return;
    }

    const history: ChatMessage[] = [
      ...entries.map((e) => ({ role: e.role, text: e.text })),
      { role: "user" as const, text: question },
    ].slice(-MAX_HISTORY_MESSAGES);

    chat.mutate({ history, generation: addQuestion(question) });
  };

  // ホームで入力した質問は、この画面を開いたときに一度だけ送る。
  // 送ったら state を消し、再読み込みや戻るで同じ質問を送り直さないようにする
  const location = useLocation();
  const navigate = useNavigate();
  const initialQuestion = (location.state as ChatLocationState | null)
    ?.question;
  const sentInitial = useRef(false);

  useEffect(() => {
    if (!initialQuestion || sentInitial.current) {
      return;
    }
    sentInitial.current = true;
    navigate(location.pathname, { replace: true, state: null });
    // 前の質問の回答を待っていたら、送らずに入力欄に入れる（捨てない）
    if (pending) {
      setChatInput(initialQuestion);
      return;
    }
    send(initialQuestion);
    // send は描画ごとに作り直されるが、ここでは最初の1回だけ呼べばよい
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialQuestion]);

  const reset = () => {
    resetChat();
    chat.reset();
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <PageTitle title="AI 乗換相談" />
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold text-foreground">AI 乗換相談</h1>

          <p className="mt-2 text-sm text-muted-foreground">
            東京都内の鉄道の経路・次の電車・運行状況を、話し言葉で聞けます。遅延や運転見合わせを確かめてから経路を提案します。
          </p>
        </div>

        {entries.length > 0 && (
          <Button variant="outline" size="sm" onClick={reset}>
            <RotateCcw />
            新しい会話
          </Button>
        )}
      </div>

      {entries.length === 0 && (
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">質問の例</p>

          <div className="flex flex-wrap gap-2">
            {EXAMPLES.map((example) => (
              <button
                key={example}
                type="button"
                onClick={() => send(example)}
                className="rounded-full border border-border px-3 py-1.5 text-left text-sm text-foreground/80 transition hover:bg-muted hover:text-foreground"
              >
                {example}
              </button>
            ))}
          </div>
        </div>
      )}

      <div className="space-y-4">
        {entries.map((entry, i) =>
          entry.role === "user" ? (
            <div key={i} className="flex justify-end">
              <p className="max-w-[85%] rounded-xl bg-primary px-4 py-2.5 whitespace-pre-wrap text-foreground">
                {entry.text}
              </p>
            </div>
          ) : (
            <AssistantMessage key={i} entry={entry} />
          ),
        )}

        {pending && (
          <div className="flex items-center gap-2 rounded-xl border border-border bg-card p-4 text-sm text-muted-foreground">
            <LoaderCircle size={16} className="animate-spin" />
            運行状況や時刻表を調べています（10秒ほどかかることがあります）
          </div>
        )}

        {error && <ChatErrorMessage error={error} />}

        <div ref={bottomRef} />
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          send(input);
        }}
        className="space-y-2"
      >
        <div className="flex items-end gap-2">
          <Textarea
            value={input}
            onChange={(e) => setChatInput(e.target.value)}
            onKeyDown={(e) => {
              // 日本語の変換を確定する Enter では送らない
              if (
                e.key === "Enter" &&
                !e.shiftKey &&
                !e.nativeEvent.isComposing
              ) {
                e.preventDefault();
                send(input);
              }
            }}
            maxLength={MAX_INPUT_CHARS}
            placeholder="行きたい場所や、知りたいことを入力"
            aria-label="質問"
            className="max-h-40 min-h-12 text-foreground"
          />

          <Button
            type="submit"
            size="icon-lg"
            disabled={!input.trim() || pending}
            aria-label="送信"
          >
            <SendHorizontal />
          </Button>
        </div>

        <div className="flex items-start justify-between gap-4 text-xs text-muted-foreground">
          <p className="flex items-start gap-1.5">
            <Info size={14} className="mt-px shrink-0" />
            入力内容は AI
            の提供元（Google）に送信され、サービス改善に利用されることがあります。個人情報は入力しないでください。
          </p>

          <span className="shrink-0 tabular-nums">
            {input.length} / {MAX_INPUT_CHARS}
          </span>
        </div>
      </form>
    </div>
  );
}
