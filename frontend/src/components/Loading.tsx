export default function Loading() {
  return (
    <div className="flex items-center justify-center py-16">
      <div className="text-center">
        <div className="mx-auto h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />

        <p className="mt-4 text-sm text-muted-foreground">読み込み中...</p>
      </div>
    </div>
  );
}
