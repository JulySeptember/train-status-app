export const APP_NAME = "NORIKAE AI";

// 名前だけでは何のサイトかわからないので、ホームの題名には中身を足す（index.html の <title> と同じ）
const HOME_TITLE = `${APP_NAME} | 東京の鉄道の運行情報・乗り換え案内（非公式）`;

// ブラウザのタブの題名。React 19 は <title> を <head> に移して表示する
export default function PageTitle({ title }: { title?: string }) {
  return <title>{title ? `${title} | ${APP_NAME}` : HOME_TITLE}</title>;
}
