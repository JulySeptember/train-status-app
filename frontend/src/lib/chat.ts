// バックエンドの上限（ai.Config.MaxInputChars / MaxHistory）と合わせる
export const MAX_INPUT_CHARS = 500;
export const MAX_HISTORY_MESSAGES = 11;

// ホームと「AI に聞く」の画面に出す質問の例。多いと読まれないので、経路と次の電車の2つに絞る
export const EXAMPLES = [
  "今春日にいる。浅草に行きたい",
  "浅草駅から次に出る電車は？",
];

// ホームで入力した質問を「AI に聞く」の画面に渡すときの、location.state の形
export type ChatLocationState = { question?: string };
