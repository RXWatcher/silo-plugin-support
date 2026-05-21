import type { LibreSpeedProgress } from "@/lib/librespeedClient";

type Props = { progress: LibreSpeedProgress | null };

export function SpeedGauge({ progress }: Props) {
  const dl = progress?.download ?? 0;
  const up = progress?.upload ?? 0;
  const pg = progress?.ping ?? 0;
  const jt = progress?.jitter ?? 0;
  const phase = progress?.phase ?? "idle";
  return (
    <div className="rounded-md border border-border bg-card p-6 space-y-3 text-center">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Stat label="Download" value={dl} unit="Mb/s" />
        <Stat label="Upload"   value={up} unit="Mb/s" />
        <Stat label="Ping"     value={pg} unit="ms" />
        <Stat label="Jitter"   value={jt} unit="ms" />
      </div>
      <p className="text-xs uppercase tracking-[0.16em] text-muted-foreground">
        {phaseLabel(phase)}
      </p>
    </div>
  );
}

function Stat({ label, value, unit }: { label: string; value: number; unit: string }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-[0.08em] text-muted-foreground">{label}</p>
      <p className="text-2xl font-semibold tabular-nums">
        {value.toFixed(value < 10 ? 2 : 1)}<span className="text-sm font-normal text-muted-foreground ml-1">{unit}</span>
      </p>
    </div>
  );
}

function phaseLabel(p: LibreSpeedProgress["phase"]): string {
  switch (p) {
    case "download": return "Downloading…";
    case "upload":   return "Uploading…";
    case "ping":     return "Measuring ping…";
    case "done":     return "Done";
    case "abort":    return "Cancelled";
    default:         return "Ready";
  }
}
