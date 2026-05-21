// Typed wrapper around LibreSpeed's speedtest_worker.js.
// The worker is loaded as a Web Worker and emits CSV-formatted
// progress messages: `<state>;<dlStatus>;<ulStatus>;<pingStatus>;<...>`.
// State: 0=idle, 1=download, 2=ping, 3=upload, 4=done, 5=abort.

export type LibreSpeedPhase = "idle" | "download" | "ping" | "upload" | "done" | "abort";

export type LibreSpeedProgress = {
  phase: LibreSpeedPhase;
  download: number;  // Mbit/s
  upload: number;
  ping: number;       // ms
  jitter: number;
};

export type LibreSpeedParams = {
  endpointURL: string;          // base URL of the LibreSpeed endpoint
  workerURL?: string;           // defaults to "./speedtest_worker.js"
  onProgress: (p: LibreSpeedProgress) => void;
  onError: (msg: string) => void;
};

export type LibreSpeedRunner = {
  start: () => void;
  abort: () => void;
};

export function createLibreSpeedRunner(params: LibreSpeedParams): LibreSpeedRunner {
  const workerURL = params.workerURL ?? "./speedtest_worker.js";
  const worker = new Worker(workerURL);
  let lastPhase: LibreSpeedPhase = "idle";

  worker.onmessage = (e: MessageEvent<string>) => {
    const parts = e.data.split(";");
    const stateRaw = parseInt(parts[0] ?? "0", 10);
    const phase = phaseFromState(stateRaw);
    const progress: LibreSpeedProgress = {
      phase,
      download: parseFloat(parts[1] ?? "0") || 0,
      upload: parseFloat(parts[2] ?? "0") || 0,
      ping: parseFloat(parts[3] ?? "0") || 0,
      jitter: parseFloat(parts[4] ?? "0") || 0,
    };
    params.onProgress(progress);
    if (phase === "done" || phase === "abort") {
      lastPhase = phase;
      worker.terminate();
    } else {
      lastPhase = phase;
    }
  };
  worker.onerror = (e) => params.onError(e.message || "speedtest worker error");

  return {
    start: () => {
      const config = {
        url_dl: params.endpointURL.replace(/\/$/, "") + "/garbage.php",
        url_ul: params.endpointURL.replace(/\/$/, "") + "/empty.php",
        url_ping: params.endpointURL.replace(/\/$/, "") + "/empty.php",
        time_dl_max: 15,
        time_ul_max: 15,
        count_ping: 10,
      };
      worker.postMessage("start " + JSON.stringify(config));
    },
    abort: () => {
      if (lastPhase !== "done" && lastPhase !== "abort") {
        worker.postMessage("abort");
      }
    },
  };
}

function phaseFromState(s: number): LibreSpeedPhase {
  switch (s) {
    case 1: return "download";
    case 2: return "ping";
    case 3: return "upload";
    case 4: return "done";
    case 5: return "abort";
    default: return "idle";
  }
}
