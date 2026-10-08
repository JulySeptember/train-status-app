import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation } from "@tanstack/react-query";
import {
  CircleCheck,
  Info,
  LoaderCircle,
  RotateCcw,
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

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

// バックエンドの上限（ai.Config.MaxInputChars / MaxHistory）と合わせる
const MAX_INPUT_CHARS = 500;
const MAX_HISTORY_MESSAGES = 11;

// 会話は sessionStorage に置き、駅のページなどへ移動して戻っても続けられるようにする
const STORAGE_KEY = "chatEntries";

const EXAMPLES = [
  "今春日にいる。浅草に行きたい",
  "新宿から浅草まで、18時までに着きたい。乗り換え少なめで",
  "浅草駅から次に出る電車は？",
  "今遅れている路線はある？",
];

// 上限に達したときなど、AI の代わりに通常の検索を案内するエラー
const FALLBACK_CODES = [
  "quota_exceeded",
  "rate_limited",
  "unavailable",
  "timeout",
];

type Entry =
  | { role: "user"; text: string }
  | {
      role: "assistant";
      text: string;
      steps: ChatStep[];
      journeys?: JourneySearch;
    };

function loadEntries(): Entry[] {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    const value: unknown = raw ? JSON.parse(raw) : [];
    return Array.isArray(value) ? (value as Entry[]) : [];
  } catch {
    return [];
  }
}

function saveEntries(entries: Entry[]) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
  } catch {
    // 保存できなくても、このページを開いている間は会話を続けられる
  }
}

function Steps({ steps }: { steps: ChatStep[] }) {
  if (steps.length === 0) {
    return null;
  }

  return (
    <ul className="space-y-1 text-sm text-gray-400">
      {steps.map((step, i) => (
        <li key={i} className="flex items-start gap-1.5">
          <CircleCheck size={14} className="mt-0.5 shrink-0 text-[#2ea043]" />
          {step.label}
        </li>
      ))}
    </ul>
  );
}

function AssistantMessage({
  entry,
}: {
  entry: Extract<Entry, { role: "assistant" }>;
}) {
  const journeys = entry.journeys;

  return (
    <div className="space-y-4 rounded-xl border border-[#30363d] bg-[#161b22] p-4">
      <Steps steps={entry.steps} />

      <p className="leading-relaxed whitespace-pre-wrap text-white">
        {entry.text}
      </p>

      {journeys && journeys.journeys.length > 0 && (
        <div className="space-y-3 border-t border-[#30363d] pt-4">
          <p className="text-xs text-gray-400">
            AI
            の回答は誤ることがあります。時刻は下の経路（時刻表のデータ）で確かめてください。
          </p>

          <RealtimeNotice data={journeys} realtime />

          <JourneyList journeys={journeys.journeys} />
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
  const [entries, setEntries] = useState<Entry[]>(loadEntries);
  const [input, setInput] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);

  const chat = useMutation<ChatResponse, Error, ChatMessage[]>({
    mutationFn: api.chat,
  });

  useEffect(() => {
    saveEntries(entries);
  }, [entries]);

  // 新しい発言・考え中の表示・エラーが出たら、そこまでスクロールする
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end", behavior: "smooth" });
  }, [entries.length, chat.isPending, chat.isError]);

  const send = (text: string) => {
    const question = text.trim();
    if (!question || chat.isPending) {
      return;
    }

    const history: ChatMessage[] = [
      ...entries.map((e) => ({ role: e.role, text: e.text })),
      { role: "user" as const, text: question },
    ].slice(-MAX_HISTORY_MESSAGES);

    setEntries((prev) => [...prev, { role: "user", text: question }]);
    setInput("");

    chat.mutate(history, {
      onSuccess: (res) => {
        setEntries((prev) => [
          ...prev,
          {
            role: "assistant",
            text: res.reply,
            steps: res.steps,
            journeys: res.journeys,
          },
        ]);
      },
      onError: () => {
        // 答えられなかった質問は履歴から外し、入力欄に戻して送り直せるようにする
        setEntries((prev) => prev.slice(0, -1));
        setInput(question);
      },
    });
  };

  const reset = () => {
    setEntries([]);
    setInput("");
    chat.reset();
  };

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold text-white">AI に聞く</h1>

          <p className="mt-2 text-sm text-gray-400">
            都営交通の経路・次の電車・運行状況を、話し言葉で聞けます。遅延や運転見合わせを確かめてから経路を提案します。
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
          <p className="text-sm text-gray-400">質問の例</p>

          <div className="flex flex-wrap gap-2">
            {EXAMPLES.map((example) => (
              <button
                key={example}
                type="button"
                onClick={() => send(example)}
                className="rounded-full border border-[#30363d] px-3 py-1.5 text-left text-sm text-gray-300 transition hover:bg-[#21262d] hover:text-white"
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
              <p className="max-w-[85%] rounded-xl bg-[#1f6feb] px-4 py-2.5 whitespace-pre-wrap text-white">
                {entry.text}
              </p>
            </div>
          ) : (
            <AssistantMessage key={i} entry={entry} />
          ),
        )}

        {chat.isPending && (
          <div className="flex items-center gap-2 rounded-xl border border-[#30363d] bg-[#161b22] p-4 text-sm text-gray-400">
            <LoaderCircle size={16} className="animate-spin" />
            運行状況や時刻表を調べています（10秒ほどかかることがあります）
          </div>
        )}

        {chat.isError && <ChatErrorMessage error={chat.error} />}

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
            onChange={(e) => setInput(e.target.value)}
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
            placeholder="例: 今春日にいる。浅草に行きたい"
            aria-label="質問"
            className="max-h-40 min-h-12 text-white"
          />

          <Button
            type="submit"
            size="icon-lg"
            disabled={!input.trim() || chat.isPending}
            aria-label="送信"
          >
            <SendHorizontal />
          </Button>
        </div>

        <div className="flex items-start justify-between gap-4 text-xs text-gray-400">
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
