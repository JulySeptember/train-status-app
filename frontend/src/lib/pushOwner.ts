// この端末の通知の購読（endpoint）が、今ログインしている利用者のものだという記録。
// Service Worker（public/sw.js）が、ブラウザが購読を作り直したとき（pushsubscriptionchange）に、
// サーバーが古い購読を消したあと（送信の 410）でも登録し直してよいかを判断するのに使う。
// ログイン・ログアウト・退会・通知の停止で消すので、記録がある間は Cookie の利用者と記録した利用者が同じ。
// 名前は sw.js と合わせる
const CACHE = "push-owner";
const KEY = "/push-owner";

// 記録を消した回数。購読の確認・登録を待つ間にログアウト・ログインし直しで記録が消えたら、
// その確認の結果（前の利用者の endpoint）を書き戻さない
let generation = 0;

export function pushOwnerGeneration(): number {
  return generation;
}

// since は、確認・登録を始める前に pushOwnerGeneration() で取った値
export async function rememberPushOwner(endpoint: string, since: number) {
  if (since !== generation) {
    return;
  }
  try {
    const cache = await caches.open(CACHE);
    if (since !== generation) {
      return;
    }
    await cache.put(KEY, new Response(JSON.stringify({ endpoint })));
  } catch {
    // 記録できなくても、SW は /status で確かめる
  }
}

export async function forgetPushOwner() {
  generation++;
  try {
    await caches.delete(CACHE);
  } catch {
    // 何もしない
  }
}
