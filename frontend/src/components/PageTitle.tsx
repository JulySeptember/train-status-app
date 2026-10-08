export const APP_NAME = "都営 運行情報";

// ブラウザのタブの題名。React 19 は <title> を <head> に移して表示する
export default function PageTitle({ title }: { title?: string }) {
  return <title>{title ? `${title} | ${APP_NAME}` : APP_NAME}</title>;
}
