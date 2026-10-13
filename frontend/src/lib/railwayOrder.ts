import { useState } from "react";

// ホームの運行情報の並び順を保存するキー。路線IDの順と、事業者のまとまりの順（事業者ID）を別に持つ
const RAILWAY_ORDER_KEY = "railwayOrder";
const OPERATOR_ORDER_KEY = "operatorOrder";

// localStorage が使えないとき（プライベートブラウズなど）や値が壊れているときは null
function load(key: string): string[] | null {
  try {
    const raw = localStorage.getItem(key);
    if (raw === null) {
      return null;
    }

    const value: unknown = JSON.parse(raw);
    if (Array.isArray(value) && value.every((v) => typeof v === "string")) {
      return value;
    }
  } catch {
    // 読めないときは API の順番で表示する
  }

  return null;
}

function save(key: string, order: string[] | null) {
  try {
    if (order === null) {
      localStorage.removeItem(key);
    } else {
      localStorage.setItem(key, JSON.stringify(order));
    }
  } catch {
    // 保存できなくても、このページを開いている間は並び替えた順で表示する
  }
}

// 保存した並び順を ids に当てはめる。保存した中に無い ID は元の順で末尾に足し、もう無い ID は捨てる
export function applyOrder(ids: string[], order: string[] | null): string[] {
  const unique = [...new Set(ids)];
  if (!order) {
    return unique;
  }

  const known = new Set(unique);
  const ordered = [...new Set(order)].filter((id) => known.has(id));
  const placed = new Set(ordered);

  return [...ordered, ...unique.filter((id) => !placed.has(id))];
}

export function sortByOrder<T>(
  items: T[],
  getId: (item: T) => string,
  order: string[],
): T[] {
  const rank = new Map(order.map((id, i) => [id, i]));
  const at = (item: T) => rank.get(getId(item)) ?? Infinity;

  return [...items].sort((a, b) => at(a) - at(b));
}

function useStoredOrder(key: string) {
  const [order, setOrder] = useState<string[] | null>(() => load(key));

  return {
    // 並び替えたことがなければ null
    order,
    setOrder(next: string[]) {
      setOrder(next);
      save(key, next);
    },
    reset() {
      setOrder(null);
      save(key, null);
    },
  };
}

export function useRailwayOrder() {
  return useStoredOrder(RAILWAY_ORDER_KEY);
}

export function useOperatorOrder() {
  return useStoredOrder(OPERATOR_ORDER_KEY);
}
