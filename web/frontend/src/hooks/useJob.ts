import { useState, useRef, useCallback, useEffect } from "react";

export type JobState = "idle" | "processing" | "done";

export interface JobParams {
  mode: number;
  count: number;
  alpha: number;
  inputSize: number;
  outputSize: number;
}

export interface JobInfo {
  width: number;
  height: number;
  background: string;
}

export interface ShapeMessage {
  svg: string;
  score: number;
  index: number;
}

export interface DoneMessage {
  totalShapes: number;
  finalScore: number;
}

export interface UseJobReturn {
  state: JobState;
  info: JobInfo | null;
  shapes: string[];
  shapeCount: number;
  totalCount: number;
  done: DoneMessage | null;
  start: (file: File, params: JobParams) => Promise<void>;
  stop: () => void;
  reset: () => void;
}

export function useJob(delay: number): UseJobReturn {
  const [state, setState] = useState<JobState>("idle");
  const [info, setInfo] = useState<JobInfo | null>(null);
  const [shapes, setShapes] = useState<string[]>([]);
  const [shapeCount, setShapeCount] = useState(0);
  const [totalCount, setTotalCount] = useState(0);
  const [doneMsg, setDoneMsg] = useState<DoneMessage | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const queueRef = useRef<{ svg: string; index: number }[]>([]);
  const pendingDoneRef = useRef<DoneMessage | null>(null);
  const drainRef = useRef<number | null>(null);
  const delayRef = useRef(delay);

  const applyShape = useCallback((svg: string, index: number) => {
    setShapes((prev) => [...prev, svg]);
    setShapeCount(index);
  }, []);

  // Keep delay ref in sync
  useEffect(() => {
    delayRef.current = delay;
  }, [delay]);

  // Manage drain timer based on delay
  useEffect(() => {
    if (drainRef.current !== null) {
      clearInterval(drainRef.current);
      drainRef.current = null;
    }

    if (delay === 0) {
      // Flush any buffered shapes immediately
      const q = queueRef.current;
      if (q.length > 0) {
        const items = q.splice(0);
        setShapes((prev) => [...prev, ...items.map((s) => s.svg)]);
        setShapeCount(items[items.length - 1].index);
      }
      // Flush pending done
      if (pendingDoneRef.current) {
        const d = pendingDoneRef.current;
        pendingDoneRef.current = null;
        setDoneMsg(d);
        setState("done");
      }
    } else {
      drainRef.current = window.setInterval(() => {
        const q = queueRef.current;
        if (q.length > 0) {
          const item = q.shift()!;
          setShapes((prev) => [...prev, item.svg]);
          setShapeCount(item.index);

          // If queue is now empty and done is pending, finalize
          if (q.length === 0 && pendingDoneRef.current) {
            const d = pendingDoneRef.current;
            pendingDoneRef.current = null;
            setDoneMsg(d);
            setState("done");
            if (drainRef.current !== null) {
              clearInterval(drainRef.current);
              drainRef.current = null;
            }
          }
        }
      }, delay);
    }

    return () => {
      if (drainRef.current !== null) {
        clearInterval(drainRef.current);
        drainRef.current = null;
      }
    };
  }, [delay]);

  const start = useCallback(
    async (file: File, params: JobParams) => {
      const formData = new FormData();
      formData.append("image", file);
      formData.append("mode", String(params.mode));
      formData.append("count", String(params.count));
      formData.append("alpha", String(params.alpha));
      formData.append("inputSize", String(params.inputSize));
      formData.append("outputSize", String(params.outputSize));

      const resp = await fetch("/api/jobs", {
        method: "POST",
        body: formData,
      });
      if (!resp.ok) {
        throw new Error(`Upload failed: ${resp.statusText}`);
      }
      const { id } = await resp.json();

      const protocol =
        window.location.protocol === "https:" ? "wss:" : "ws:";
      const ws = new WebSocket(
        `${protocol}//${window.location.host}/api/ws/${id}`
      );
      wsRef.current = ws;

      ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        switch (msg.type) {
          case "started":
            setInfo({
              width: msg.width,
              height: msg.height,
              background: msg.background,
            });
            setTotalCount(msg.count);
            setState("processing");
            break;
          case "shape":
            if (delayRef.current === 0) {
              applyShape(msg.svg, msg.index);
            } else {
              queueRef.current.push({ svg: msg.svg, index: msg.index });
            }
            break;
          case "done":
            if (
              delayRef.current === 0 ||
              queueRef.current.length === 0
            ) {
              setDoneMsg({
                totalShapes: msg.totalShapes,
                finalScore: msg.finalScore,
              });
              setState("done");
              wsRef.current = null;
            } else {
              pendingDoneRef.current = {
                totalShapes: msg.totalShapes,
                finalScore: msg.finalScore,
              };
              wsRef.current = null;
            }
            break;
        }
      };

      ws.onerror = () => {
        if (drainRef.current !== null) {
          clearInterval(drainRef.current);
          drainRef.current = null;
        }
        queueRef.current = [];
        pendingDoneRef.current = null;
        setState("done");
        wsRef.current = null;
      };
    },
    [applyShape]
  );

  const stop = useCallback(() => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "stop" }));
    }
  }, []);

  const reset = useCallback(() => {
    const ws = wsRef.current;
    if (ws) {
      ws.close();
      wsRef.current = null;
    }
    if (drainRef.current !== null) {
      clearInterval(drainRef.current);
      drainRef.current = null;
    }
    queueRef.current = [];
    pendingDoneRef.current = null;
    setState("idle");
    setInfo(null);
    setShapes([]);
    setShapeCount(0);
    setTotalCount(0);
    setDoneMsg(null);
  }, []);

  return {
    state,
    info,
    shapes,
    shapeCount,
    totalCount,
    done: doneMsg,
    start,
    stop,
    reset,
  };
}
