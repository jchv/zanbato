const isWorker =
  typeof window === "undefined" &&
  typeof self !== "undefined" &&
  typeof (globalThis as { WorkerGlobalScope?: unknown }).WorkerGlobalScope !==
    "undefined";

const enabledNamespaces = new Set<string>();
let wildcardEnabled = false;

const DEBUG_KEY = "zanbato_debug";

function parseFilter(spec: string | null | undefined): void {
  if (!spec) return;
  for (const ns of spec
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)) {
    if (ns === "*") wildcardEnabled = true;
    else enabledNamespaces.add(ns);
  }
}

if (typeof localStorage !== "undefined") {
  try {
    parseFilter(localStorage.getItem(DEBUG_KEY));
  } catch {
    // localStorage may throw if storage is full or disabled - ignore.
  }
}

function isEnabled(namespace: string): boolean {
  if (isWorker) return true;
  return wildcardEnabled || enabledNamespaces.has(namespace);
}

export interface Logger {
  debug: (...args: unknown[]) => void;
  info: (...args: unknown[]) => void;
  warn: (...args: unknown[]) => void;
  error: (...args: unknown[]) => void;
}

export function createLogger(namespace: string): Logger {
  const prefix = `[${namespace}]`;
  return {
    debug: (...args) => {
      if (isEnabled(namespace)) console.log(prefix, ...args);
    },
    info: (...args) => console.info(prefix, ...args),
    warn: (...args) => console.warn(prefix, ...args),
    error: (...args) => console.error(prefix, ...args),
  };
}
