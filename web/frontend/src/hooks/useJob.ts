import { useState, useRef, useCallback } from "react";

export type JobState = "idle" | "processing" | "done";

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
  start: (file: File) => Promise<void>;
  stop: () => void;
  reset: () => void;
}

export function useJob(): UseJobReturn {
  const [state, setState] = useState<JobState>("idle");
  const [info, setInfo] = useState<JobInfo | null>(null);
  const [shapes, setShapes] = useState<string[]>([]);
  const [shapeCount, setShapeCount] = useState(0);
  const [totalCount, setTotalCount] = useState(0);
  const [doneMsg, setDoneMsg] = useState<DoneMessage | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const start = useCallback(async (file: File) => {
    // Upload image
    const formData = new FormData();
    formData.append("image", file);

    const resp = await fetch("/api/jobs", { method: "POST", body: formData });
    if (!resp.ok) {
      throw new Error(`Upload failed: ${resp.statusText}`);
    }
    const { id } = await resp.json();

    // Connect WebSocket
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${protocol}//${window.location.host}/api/ws/${id}`);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      switch (msg.type) {
        case "started":
          setInfo({ width: msg.width, height: msg.height, background: msg.background });
          setTotalCount(msg.count);
          setState("processing");
          break;
        case "shape":
          setShapes((prev) => [...prev, msg.svg]);
          setShapeCount(msg.index);
          break;
        case "done":
          setDoneMsg({ totalShapes: msg.totalShapes, finalScore: msg.finalScore });
          setState("done");
          wsRef.current = null;
          break;
      }
    };

    ws.onerror = () => {
      setState("done");
      wsRef.current = null;
    };
  }, []);

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
    setState("idle");
    setInfo(null);
    setShapes([]);
    setShapeCount(0);
    setTotalCount(0);
    setDoneMsg(null);
  }, []);

  return { state, info, shapes, shapeCount, totalCount, done: doneMsg, start, stop, reset };
}
