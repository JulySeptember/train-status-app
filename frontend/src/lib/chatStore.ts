import { useSyncExternalStore } from "react";

import { type ChatStep, type JourneySearch } from "@/types";

// AI 乗換相談の会話。回答を待つ間に別の画面へ移っても、回答を会話に足せるよう、
// 画面（Chat）の外に置き、sessionStorage にも保存する（駅のページなどへ移動して戻っても続けられるように）

export type ChatEntry =
  | { role: "user"; text: string }
  | {
      role: "assistant";
      text: string;
      steps: ChatStep[];
      journeys?: JourneySearch;
    };

type ChatState = {
  entries: ChatEntry[];

  // 「新しい会話」で増える。前の会話の質問への回答を、新しい会話に足さないため
  generation: number;

  // 入力欄の文字。答えられなかった質問もここに戻す（画面を離れている間に失敗したときも）
  input: string;
};

const STORAGE_KEY = "chatEntries";

function load(): ChatState {
  let entries: ChatEntry[] = [];
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    const value: unknown = raw ? JSON.parse(raw) : [];
    entries = Array.isArray(value) ? (value as ChatEntry[]) : [];
  } catch {
    // 読めなければ新しい会話にする
  }

  // 読み込み直したときに、答えの無い質問が最後に残っていたら（回答を待つ間に再読み込みした）、
  // 履歴から外して入力欄に戻す。残すと user の発言が2つ続いた履歴を送ってしまう
  const last = entries.at(-1);
  if (last?.role === "user") {
    return { entries: entries.slice(0, -1), generation: 0, input: last.text };
  }

  return { entries, generation: 0, input: "" };
}

let state: ChatState | undefined;
const listeners = new Set<() => void>();

function current(): ChatState {
  state ??= load();
  return state;
}

function update(next: ChatState) {
  const saved = state?.entries === next.entries;
  state = next;
  if (!saved) {
    try {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(next.entries));
    } catch {
      // 保存できなくても、ページを開いている間は会話を続けられる
    }
  }
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useChatState(): ChatState {
  return useSyncExternalStore(subscribe, current);
}

export function setChatInput(input: string) {
  update({ ...current(), input });
}

// addQuestion は質問を会話に足して入力欄を空にし、その会話の generation を返す
export function addQuestion(text: string): number {
  const s = current();
  update({ ...s, entries: [...s.entries, { role: "user", text }], input: "" });
  return s.generation;
}

// addAnswer は回答を足す。質問のあとに「新しい会話」にしていたら足さない
export function addAnswer(
  generation: number,
  entry: Extract<ChatEntry, { role: "assistant" }>,
) {
  const s = current();
  if (s.generation !== generation) {
    return;
  }
  update({ ...s, entries: [...s.entries, entry] });
}

// dropQuestion は、答えられなかった質問を履歴から外し、入力欄に戻して送り直せるようにする
export function dropQuestion(generation: number, text: string) {
  const s = current();
  const last = s.entries.at(-1);
  if (s.generation !== generation || last?.role !== "user") {
    return;
  }
  update({ ...s, entries: s.entries.slice(0, -1), input: text });
}

export function resetChat() {
  const s = current();
  update({ entries: [], generation: s.generation + 1, input: "" });
}
