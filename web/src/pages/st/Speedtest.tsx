import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { EndpointPicker } from "@/components/st/EndpointPicker";
import { HistoryList } from "@/components/st/HistoryList";
import { SpeedGauge } from "@/components/st/SpeedGauge";
import { TopBar } from "@/components/shared/TopBar";
import { Button } from "@/components/ui/button";
import {
  getSTAuto, getSTHistory, listSTEndpoints, saveSTResult,
} from "@/api/st";
import {
  createLibreSpeedRunner, type LibreSpeedProgress, type LibreSpeedRunner,
} from "@/lib/librespeedClient";
import type { STAutoResolution, STEndpoint, STResult } from "@/lib/types";

type Choice = "auto" | number;

export function Speedtest() {
  const [endpoints, setEndpoints] = useState<STEndpoint[]>([]);
  const [history, setHistory] = useState<STResult[]>([]);
  const [choice, setChoice] = useState<Choice>("auto");
  const [autoRes, setAutoRes] = useState<STAutoResolution | null>(null);
  const [progress, setProgress] = useState<LibreSpeedProgress | null>(null);
  const [running, setRunning] = useState(false);
  const runnerRef = useRef<LibreSpeedRunner | null>(null);

  useEffect(() => {
    listSTEndpoints().then(setEndpoints).catch(() => {});
    getSTHistory().then(setHistory).catch(() => {});
    getSTAuto().then(setAutoRes).catch(() => {});
  }, []);

  function resolveEndpoint(): { endpoint: STEndpoint; strategy: string } | null {
    if (choice === "auto") {
      if (autoRes?.endpoint) {
        return { endpoint: autoRes.endpoint, strategy: autoRes.strategy };
      }
      // Latency mode: SPA picks the lowest-RTT candidate via parallel HEADs.
      // For v1 we fall back to "first candidate" if the latency probe
      // hasn't completed — a future iteration can run the probe here.
      const first = autoRes?.candidates?.[0];
      if (first) return { endpoint: first, strategy: "latency" };
      return null;
    }
    const ep = endpoints.find((e) => e.id === choice);
    return ep ? { endpoint: ep, strategy: "" } : null;
  }

  function runTest() {
    const resolved = resolveEndpoint();
    if (!resolved) {
      toast.error("No endpoint available — ask your admin to configure one.");
      return;
    }
    setRunning(true);
    setProgress({ phase: "idle", download: 0, upload: 0, ping: 0, jitter: 0 });
    runnerRef.current = createLibreSpeedRunner({
      endpointURL: resolved.endpoint.url,
      workerURL: "../speedtest_worker.js",
      onProgress: async (p) => {
        setProgress(p);
        if (p.phase === "done") {
          try {
            const saved = await saveSTResult({
              endpointId: resolved.endpoint.id,
              endpointLabel: resolved.endpoint.label,
              autoStrategy: choice === "auto" ? resolved.strategy : "",
              downloadMbps: p.download,
              uploadMbps: p.upload,
              pingMs: p.ping,
              jitterMs: p.jitter,
            });
            setHistory((h) => [saved, ...h]);
            toast.success("Test saved.");
          } catch (err) {
            toast.error(err instanceof Error ? err.message : "Save failed");
          } finally {
            setRunning(false);
          }
        }
        if (p.phase === "abort") {
          setRunning(false);
          toast.info("Test cancelled.");
        }
      },
      onError: (msg) => {
        toast.error(msg);
        setRunning(false);
      },
    });
    runnerRef.current.start();
  }

  function abortTest() {
    runnerRef.current?.abort();
  }

  return (
    <main className="min-h-[100dvh] bg-background text-foreground">
      <div className="mx-auto max-w-3xl space-y-5 px-4 py-10 md:px-8">
        <TopBar
          eyebrow="Support"
          title="Speedtest"
          subtitle="Test your connection against our endpoints."
        />
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground">Run against</span>
          <EndpointPicker endpoints={endpoints} value={choice} onChange={setChoice} disabled={running} />
          {!running && <Button onClick={runTest}>Run test</Button>}
          {running && <Button variant="destructive" onClick={abortTest}>Cancel</Button>}
        </div>
        {choice === "auto" && autoRes?.endpoint && (
          <p className="text-xs text-muted-foreground">
            Auto: {autoRes.endpoint.label}
            {autoRes.strategy === "geoip" && autoRes.geoip?.country
              ? ` · selected by geoip (${autoRes.geoip.country})` : ""}
            {autoRes.strategy === "latency" ? " · selected by latency" : ""}
            {autoRes.strategy === "fallback" ? " · fallback (no geoip / latency data)" : ""}
          </p>
        )}
        <SpeedGauge progress={progress} />
        <div>
          <h2 className="mb-2 text-sm font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            Your last 5 tests
          </h2>
          <HistoryList history={history} />
        </div>
      </div>
    </main>
  );
}
