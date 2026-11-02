import { useEffect, type ComponentType } from "react";

import Error from "@/components/Error";
import Loading from "@/components/Loading";

// 遅延読み込みに失敗して、ページを読み直したことの目印（読み直しを繰り返さないため）
const RELOADED_KEY = "lazyPageReloaded";

// 読み直したことがあるか。無ければ目印を付ける（sessionStorage を使えなければ、読み直さない）
function markReloaded(): boolean {
  try {
    if (sessionStorage.getItem(RELOADED_KEY)) {
      return true;
    }
    sessionStorage.setItem(RELOADED_KEY, "1");
    return false;
  } catch {
    return true;
  }
}

// 目印を消す（次のデプロイのあとにも、1回は読み直せるように）
function clearReloaded() {
  try {
    sessionStorage.removeItem(RELOADED_KEY);
  } catch {
    // 消せなくても、次に失敗したときに読み直さないだけ
  }
}

// 読み直して新しい版を取る画面。読み直すまでは読み込み中にする
function ReloadPage() {
  useEffect(() => {
    window.location.reload();
  }, []);

  return <Loading />;
}

// 読み込めなかったときの画面（「再読み込み」のボタンで読み直せる）。
// React Router は lazy の結果を覚えるので、ページを読み直すまでこのルートはこの画面のまま。
// 自動の読み直しはここで終わりなので目印を消し、次のデプロイのあとにも1回は自動で読み直せるようにする
function LoadFailed() {
  useEffect(() => {
    clearReloaded();
  }, []);

  return <Error message="画面を読み込めませんでした。" />;
}

// 画面のモジュールを読み込む（React Router の lazy から呼ぶ）。
// デプロイで前の版の JS が消えたあと、開いたままのタブで開くと読み込めないので、
// その画面の URL のまま1回だけ読み直して新しい版を取る。読み直しても駄目ならエラーを出す。
// オフラインのときは、読み直すとブラウザのオフラインの画面になり今の画面も失うので、読み直さずにエラーを出す。
// 失敗しても投げずに、代わりの画面を返す（lazy が投げると、React Router は route.lazy が残ったままになり、
// errorElement を出さずに Outlet を空にする）
export async function loadLazyPage(
  load: () => Promise<{ default: ComponentType }>,
): Promise<ComponentType> {
  try {
    const page = await load();
    clearReloaded();
    return page.default;
  } catch {
    if (!navigator.onLine) {
      return LoadFailed;
    }
    return markReloaded() ? LoadFailed : ReloadPage;
  }
}
