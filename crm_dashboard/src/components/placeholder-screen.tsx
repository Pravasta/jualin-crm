// Temporary. The app shell (issue #40) ships all six nav destinations at
// once, but the screens behind them land one issue at a time (#32–#35) —
// without these, five nav items would 404 for several PR cycles.
//
// Each of these is deleted outright by the issue that builds its screen.
// Naming the issue here is what keeps that from being forgotten.
export function PlaceholderScreen({
  title,
  description,
  issue,
}: {
  title: string;
  description: string;
  issue: number;
}) {
  return (
    <div className="flex min-h-[50vh] flex-col items-center justify-center gap-2 text-center">
      <h2 className="text-base font-medium">{title}</h2>
      <p className="max-w-md text-sm text-muted-foreground">{description}</p>
      <p className="mt-2 text-xs text-muted-foreground">
        Layar ini dibangun di issue #{issue}.
      </p>
    </div>
  );
}
