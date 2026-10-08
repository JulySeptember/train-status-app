// バックエンドの上限（ai.Config.MaxInputChars / MaxHistory）と合わせる
export const MAX_INPUT_CHARS = 500;
export const MAX_HISTORY_MESSAGES = 11;

// ホームと「AI に聞く」の画面に出す質問の例
export const EXAMPLES = [
  "今春日にいる。浅草に行きたい",
  "新宿から浅草まで、18時までに着きたい。乗り換え少なめで",
  "浅草駅から次に出る電車は？",
  "今遅れている路線はある？",
];

// ホームで入力した質問を「AI に聞く」の画面に渡すときの、location.state の形
export type ChatLocationState = { question?: string };
