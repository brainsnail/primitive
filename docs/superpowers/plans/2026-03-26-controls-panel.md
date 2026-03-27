# Controls Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a configuration screen between image upload and processing, letting users set shape type, count, alpha, input/output resolution, and playback speed.

**Architecture:** New `ControlPanel` component sits between upload and processing in the UI flow. Backend gains `InputSize`/`OutputSize` config fields. Client-side message buffering in `useJob` implements playback delay — server runs at full speed, frontend drains a queue on a timer. No WebSocket protocol changes.

**Tech Stack:** Go (backend), React 19 + TypeScript (frontend), existing `primitive` library

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `web/server/job.go` | Modify | Add `InputSize`/`OutputSize` to `JobConfig`, use in `NewJob` |
| `web/server/job_test.go` | Modify | Test custom input/output sizes |
| `web/server/server.go` | Modify | Parse `inputSize`/`outputSize` form fields, fix mode/alpha zero-value defaults |
| `web/server/server_test.go` | Modify | Test new form fields, test mode=0 works |
| `web/frontend/src/hooks/useJob.ts` | Modify | Add `JobParams`, update `start()` signature, add delay buffering |
| `web/frontend/src/components/ControlPanel.tsx` | Create | Config form with thumbnail preview |
| `web/frontend/src/App.tsx` | Modify | New `file` state, configure screen routing, wire `ControlPanel` |
| `web/frontend/src/App.css` | Modify | Add control panel styles |

---

### Task 1: Backend — Add InputSize/OutputSize to JobConfig and NewJob

**Files:**
- Modify: `web/server/job.go:33-37` (JobConfig struct)
- Modify: `web/server/job.go:68-88` (NewJob function)
- Modify: `web/server/job_test.go`

- [ ] **Step 1: Write the failing test**

Add to `web/server/job_test.go`:

```go
func TestNewJobCustomSizes(t *testing.T) {
	img := testImage(512, 512)
	job := NewJob(img, JobConfig{
		Mode:       1,
		Count:      5,
		Alpha:      128,
		InputSize:  128,
		OutputSize: 512,
	})
	if job.Info.Width != 512 || job.Info.Height != 512 {
		t.Fatalf("expected 512x512 output, got %dx%d", job.Info.Width, job.Info.Height)
	}
}

func TestNewJobDefaultSizes(t *testing.T) {
	img := testImage(512, 512)
	job := NewJob(img, JobConfig{
		Mode:  1,
		Count: 5,
		Alpha: 128,
	})
	// Default outputSize=1024, so scaled output should be 1024x1024 for square input
	if job.Info.Width != 1024 || job.Info.Height != 1024 {
		t.Fatalf("expected 1024x1024 default output, got %dx%d", job.Info.Width, job.Info.Height)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/logan/develop/primitive && go test ./web/server/ -run "TestNewJobCustomSizes|TestNewJobDefaultSizes" -v`
Expected: FAIL — `JobConfig` has no field `InputSize`

- [ ] **Step 3: Add InputSize/OutputSize to JobConfig**

In `web/server/job.go`, replace the `JobConfig` struct:

```go
// JobConfig holds the parameters for a primitive job.
type JobConfig struct {
	Mode       int `json:"mode"`
	Count      int `json:"count"`
	Alpha      int `json:"alpha"`
	InputSize  int `json:"inputSize"`
	OutputSize int `json:"outputSize"`
}
```

- [ ] **Step 4: Update NewJob to use configurable sizes**

In `web/server/job.go`, replace the `NewJob` function:

```go
// NewJob creates a job from an uploaded image and config.
// InputSize controls the processing resolution (default 256).
// OutputSize controls the rendered SVG resolution (default 1024).
func NewJob(img image.Image, config JobConfig) *Job {
	inputSize := config.InputSize
	if inputSize == 0 {
		inputSize = 256
	}
	outputSize := config.OutputSize
	if outputSize == 0 {
		outputSize = 1024
	}

	input := resize.Thumbnail(uint(inputSize), uint(inputSize), img, resize.Bilinear)

	bg := primitive.MakeColor(primitive.AverageImageColor(input))
	model := primitive.NewModel(input, bg, outputSize, runtime.NumCPU())

	bgHex := fmt.Sprintf("#%02x%02x%02x", bg.R, bg.G, bg.B)

	return &Job{
		ID:     generateID(),
		Status: StatusPending,
		Config: config,
		Info: JobInfo{
			Width:      model.Sw,
			Height:     model.Sh,
			Background: bgHex,
		},
		model:  model,
		stopCh: make(chan struct{}),
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/logan/develop/primitive && go test ./web/server/ -run "TestNewJobCustomSizes|TestNewJobDefaultSizes" -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `cd /Users/logan/develop/primitive && go test ./web/server/ -v`
Expected: All tests pass (existing tests still work since they omit InputSize/OutputSize, which default to 256/1024)

- [ ] **Step 7: Commit**

```bash
git add web/server/job.go web/server/job_test.go
git commit -m "feat: add InputSize/OutputSize to JobConfig and NewJob"
```

---

### Task 2: Backend — Parse new form fields and fix zero-value defaults

**Files:**
- Modify: `web/server/server.go:50-95` (handleCreateJob)
- Modify: `web/server/server_test.go`

- [ ] **Step 1: Write failing tests**

Add to `web/server/server_test.go`:

```go
func TestCreateJobWithSizes(t *testing.T) {
	srv := NewServer()
	imgBytes := encodePNG(testImage(64, 64))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("image", "test.png")
	part.Write(imgBytes)
	writer.WriteField("inputSize", "128")
	writer.WriteField("outputSize", "512")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		body, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, body)
	}

	var result struct{ ID string `json:"id"` }
	json.NewDecoder(w.Result().Body).Decode(&result)

	srv.jobsMu.RLock()
	job := srv.jobs[result.ID]
	srv.jobsMu.RUnlock()

	if job.Config.InputSize != 128 {
		t.Fatalf("expected inputSize 128, got %d", job.Config.InputSize)
	}
	if job.Config.OutputSize != 512 {
		t.Fatalf("expected outputSize 512, got %d", job.Config.OutputSize)
	}
}

func TestCreateJobMode0(t *testing.T) {
	srv := NewServer()
	imgBytes := encodePNG(testImage(64, 64))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("image", "test.png")
	part.Write(imgBytes)
	writer.WriteField("mode", "0")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var result struct{ ID string `json:"id"` }
	json.NewDecoder(w.Result().Body).Decode(&result)

	srv.jobsMu.RLock()
	job := srv.jobs[result.ID]
	srv.jobsMu.RUnlock()

	if job.Config.Mode != 0 {
		t.Fatalf("expected mode 0 (Combo), got %d", job.Config.Mode)
	}
}

func TestCreateJobSizeClamping(t *testing.T) {
	srv := NewServer()
	imgBytes := encodePNG(testImage(64, 64))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("image", "test.png")
	part.Write(imgBytes)
	writer.WriteField("inputSize", "9999")
	writer.WriteField("outputSize", "10")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var result struct{ ID string `json:"id"` }
	json.NewDecoder(w.Result().Body).Decode(&result)

	srv.jobsMu.RLock()
	job := srv.jobs[result.ID]
	srv.jobsMu.RUnlock()

	if job.Config.InputSize != 1024 {
		t.Fatalf("expected inputSize clamped to 1024, got %d", job.Config.InputSize)
	}
	if job.Config.OutputSize != 256 {
		t.Fatalf("expected outputSize clamped to 256, got %d", job.Config.OutputSize)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/logan/develop/primitive && go test ./web/server/ -run "TestCreateJobWithSizes|TestCreateJobMode0|TestCreateJobSizeClamping" -v`
Expected: `TestCreateJobWithSizes` — inputSize is 0 (not parsed). `TestCreateJobMode0` — mode is 1 (overridden from 0).

- [ ] **Step 3: Rewrite handleCreateJob form parsing**

In `web/server/server.go`, replace the form value parsing section (lines 70–86) inside `handleCreateJob` with:

```go
	// Parse config with defaults — check string emptiness so that
	// valid zero values (mode=0 for Combo, alpha=0 for Auto) are not overridden.
	mode := 1
	if s := r.FormValue("mode"); s != "" {
		mode, _ = strconv.Atoi(s)
	}

	count := 100
	if s := r.FormValue("count"); s != "" {
		count, _ = strconv.Atoi(s)
		if count < 1 {
			count = 1
		}
		if count > 1000 {
			count = 1000
		}
	}

	alpha := 128
	if s := r.FormValue("alpha"); s != "" {
		alpha, _ = strconv.Atoi(s)
	}

	inputSize := 256
	if s := r.FormValue("inputSize"); s != "" {
		inputSize, _ = strconv.Atoi(s)
		if inputSize < 64 {
			inputSize = 64
		}
		if inputSize > 1024 {
			inputSize = 1024
		}
	}

	outputSize := 1024
	if s := r.FormValue("outputSize"); s != "" {
		outputSize, _ = strconv.Atoi(s)
		if outputSize < 256 {
			outputSize = 256
		}
		if outputSize > 4096 {
			outputSize = 4096
		}
	}

	job := NewJob(img, JobConfig{
		Mode:       mode,
		Count:      count,
		Alpha:      alpha,
		InputSize:  inputSize,
		OutputSize: outputSize,
	})
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/logan/develop/primitive && go test ./web/server/ -run "TestCreateJobWithSizes|TestCreateJobMode0|TestCreateJobSizeClamping" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `cd /Users/logan/develop/primitive && go test ./web/server/ -v`
Expected: All tests pass

- [ ] **Step 6: Commit**

```bash
git add web/server/server.go web/server/server_test.go
git commit -m "feat: parse inputSize/outputSize form fields, fix mode/alpha zero-value defaults"
```

---

### Task 3: Frontend — Add JobParams and delay buffering to useJob

**Files:**
- Modify: `web/frontend/src/hooks/useJob.ts`

- [ ] **Step 1: Replace useJob.ts with updated version**

Replace the entire contents of `web/frontend/src/hooks/useJob.ts` with:

```ts
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
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/logan/develop/primitive/web/frontend && npx tsc --noEmit`
Expected: Type errors in `App.tsx` because `useJob` now requires a `delay` argument and `start` requires `JobParams`. This is expected — App.tsx is updated in Task 5.

- [ ] **Step 3: Commit**

```bash
git add web/frontend/src/hooks/useJob.ts
git commit -m "feat: add JobParams to useJob.start() and delay buffering for playback speed"
```

---

### Task 4: Frontend — Create ControlPanel component

**Files:**
- Create: `web/frontend/src/components/ControlPanel.tsx`

- [ ] **Step 1: Create the ControlPanel component**

Create `web/frontend/src/components/ControlPanel.tsx`:

```tsx
import { useState, useMemo } from "react";
import type { JobParams } from "../hooks/useJob";

interface ControlPanelProps {
  file: File;
  onStart: (params: JobParams, delay: number) => void;
  onBack: () => void;
}

const SHAPE_TYPES = [
  { value: 0, label: "Combo" },
  { value: 1, label: "Triangle" },
  { value: 2, label: "Rectangle" },
  { value: 3, label: "Ellipse" },
  { value: 4, label: "Circle" },
  { value: 5, label: "Rotated Rect" },
  { value: 6, label: "Bezier" },
  { value: 7, label: "Rotated Ellipse" },
  { value: 8, label: "Polygon" },
];

export function ControlPanel({ file, onStart, onBack }: ControlPanelProps) {
  const [mode, setMode] = useState(1);
  const [count, setCount] = useState(100);
  const [alpha, setAlpha] = useState(128);
  const [autoAlpha, setAutoAlpha] = useState(false);
  const [inputSize, setInputSize] = useState(256);
  const [outputSize, setOutputSize] = useState(1024);
  const [delay, setDelay] = useState(0);

  const thumbnailUrl = useMemo(() => URL.createObjectURL(file), [file]);

  const handleStart = () => {
    onStart(
      { mode, count, alpha: autoAlpha ? 0 : alpha, inputSize, outputSize },
      delay
    );
  };

  return (
    <div className="control-panel">
      <div className="control-panel__preview">
        <img
          src={thumbnailUrl}
          alt="Preview"
          className="control-panel__thumbnail"
        />
      </div>
      <div className="control-panel__form">
        <div className="control-group">
          <label className="control-label">Shape Type</label>
          <select
            className="control-select"
            value={mode}
            onChange={(e) => setMode(Number(e.target.value))}
          >
            {SHAPE_TYPES.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </div>

        <div className="control-group">
          <label className="control-label">Shape Count</label>
          <input
            type="number"
            className="control-input"
            value={count}
            min={1}
            max={1000}
            onChange={(e) =>
              setCount(Math.max(1, Math.min(1000, Number(e.target.value))))
            }
          />
        </div>

        <div className="control-group">
          <label className="control-label">Alpha</label>
          <div className="control-row">
            <input
              type="range"
              className="control-slider"
              value={alpha}
              min={1}
              max={255}
              disabled={autoAlpha}
              onChange={(e) => setAlpha(Number(e.target.value))}
            />
            <input
              type="number"
              className="control-input control-input--small"
              value={alpha}
              min={1}
              max={255}
              disabled={autoAlpha}
              onChange={(e) =>
                setAlpha(Math.max(1, Math.min(255, Number(e.target.value))))
              }
            />
            <label className="control-checkbox">
              <input
                type="checkbox"
                checked={autoAlpha}
                onChange={(e) => setAutoAlpha(e.target.checked)}
              />
              Auto
            </label>
          </div>
        </div>

        <div className="control-group">
          <label className="control-label">Input Size</label>
          <input
            type="number"
            className="control-input"
            value={inputSize}
            min={64}
            max={1024}
            onChange={(e) =>
              setInputSize(Math.max(64, Math.min(1024, Number(e.target.value))))
            }
          />
          <span className="control-hint">
            Processing resolution — smaller = faster
          </span>
        </div>

        <div className="control-group">
          <label className="control-label">Output Size</label>
          <input
            type="number"
            className="control-input"
            value={outputSize}
            min={256}
            max={4096}
            onChange={(e) =>
              setOutputSize(
                Math.max(256, Math.min(4096, Number(e.target.value)))
              )
            }
          />
          <span className="control-hint">Render resolution</span>
        </div>

        <div className="control-group">
          <label className="control-label">
            Playback Speed
            <span className="control-value">
              {delay === 0 ? "Instant" : delay >= 500 ? "Slow" : `${delay}ms`}
            </span>
          </label>
          <input
            type="range"
            className="control-slider"
            value={delay}
            min={0}
            max={500}
            onChange={(e) => setDelay(Number(e.target.value))}
          />
          <div className="control-range-labels">
            <span>Instant</span>
            <span>Slow</span>
          </div>
        </div>

        <div className="control-panel__buttons">
          <button className="btn btn--secondary" onClick={onBack}>
            Back
          </button>
          <button className="btn btn--primary" onClick={handleStart}>
            Start
          </button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verify it compiles (component only)**

Run: `cd /Users/logan/develop/primitive/web/frontend && npx tsc --noEmit 2>&1 | grep -v "App.tsx"`
Expected: No errors from ControlPanel.tsx itself (App.tsx errors expected until Task 5)

- [ ] **Step 3: Commit**

```bash
git add web/frontend/src/components/ControlPanel.tsx
git commit -m "feat: add ControlPanel component with shape, size, and playback controls"
```

---

### Task 5: Frontend — Wire ControlPanel into App and add styles

**Files:**
- Modify: `web/frontend/src/App.tsx`
- Modify: `web/frontend/src/App.css`

- [ ] **Step 1: Replace App.tsx with updated version**

Replace the entire contents of `web/frontend/src/App.tsx` with:

```tsx
import { useState, useCallback } from "react";
import "./App.css";
import { useJob } from "./hooks/useJob";
import type { JobParams } from "./hooks/useJob";
import { UploadZone } from "./components/UploadZone";
import { ControlPanel } from "./components/ControlPanel";
import { Canvas } from "./components/Canvas";
import { Toolbar } from "./components/Toolbar";

function App() {
  const [file, setFile] = useState<File | null>(null);
  const [delay, setDelay] = useState(0);
  const job = useJob(delay);

  const handleFile = useCallback((f: File) => {
    setFile(f);
  }, []);

  const handleStart = useCallback(
    (params: JobParams, playbackDelay: number) => {
      if (!file) return;
      setDelay(playbackDelay);
      job.start(file, params);
      setFile(null);
    },
    [file, job.start]
  );

  const handleBack = useCallback(() => {
    setFile(null);
  }, []);

  const handleReset = useCallback(() => {
    job.reset();
    setDelay(0);
  }, [job.reset]);

  const handleDownloadSVG = useCallback(() => {
    if (!job.info) return;
    const svgEl = document.querySelector(".canvas-svg");
    if (!svgEl) return;

    const svgData = new XMLSerializer().serializeToString(svgEl);
    const blob = new Blob([svgData], { type: "image/svg+xml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "primitive.svg";
    a.click();
    URL.revokeObjectURL(url);
  }, [job.info]);

  const handleDownloadPNG = useCallback(() => {
    if (!job.info) return;
    const svgEl = document.querySelector(".canvas-svg");
    if (!svgEl) return;

    const svgData = new XMLSerializer().serializeToString(svgEl);
    const canvas = document.createElement("canvas");
    canvas.width = job.info.width;
    canvas.height = job.info.height;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const img = new Image();
    img.onload = () => {
      ctx.drawImage(img, 0, 0);
      canvas.toBlob((blob) => {
        if (!blob) return;
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "primitive.png";
        a.click();
        URL.revokeObjectURL(url);
      }, "image/png");
    };
    img.src = "data:image/svg+xml;base64," + btoa(svgData);
  }, [job.info]);

  return (
    <div className="app">
      <header className="header">
        <h1>PRIMITIVE</h1>
      </header>
      <main className="main">
        {job.state === "idle" && !file && <UploadZone onFile={handleFile} />}
        {job.state === "idle" && file && (
          <ControlPanel
            file={file}
            onStart={handleStart}
            onBack={handleBack}
          />
        )}
        {job.state !== "idle" && job.info && (
          <div className="canvas-wrapper">
            <Canvas
              info={job.info}
              shapes={job.shapes}
              shapeCount={job.shapeCount}
              totalCount={job.totalCount}
            />
            <Toolbar
              state={job.state}
              onStop={job.stop}
              onReset={handleReset}
              onDownloadSVG={handleDownloadSVG}
              onDownloadPNG={handleDownloadPNG}
            />
          </div>
        )}
      </main>
    </div>
  );
}

export default App;
```

- [ ] **Step 2: Add control panel CSS**

Append the following to the end of `web/frontend/src/App.css`:

```css
/* Control Panel */
.control-panel {
  display: flex;
  gap: 32px;
  max-width: 800px;
  width: 100%;
  background: #16213e;
  border-radius: 12px;
  padding: 24px;
  border: 1px solid #333;
}

@media (max-width: 640px) {
  .control-panel {
    flex-direction: column;
  }
}

.control-panel__preview {
  flex: 0 0 auto;
  max-width: 280px;
}

.control-panel__thumbnail {
  width: 100%;
  border-radius: 8px;
  display: block;
}

.control-panel__form {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.control-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.control-label {
  font-size: 13px;
  color: #aaa;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.control-value {
  color: #4fc3f7;
  font-size: 12px;
}

.control-select,
.control-input {
  background: #1a1a2e;
  border: 1px solid #444;
  border-radius: 4px;
  color: #e0e0e0;
  padding: 6px 10px;
  font-size: 14px;
}

.control-select:focus,
.control-input:focus {
  outline: none;
  border-color: #4fc3f7;
}

.control-input--small {
  width: 64px;
}

.control-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.control-slider {
  flex: 1;
  accent-color: #4fc3f7;
}

.control-checkbox {
  font-size: 13px;
  color: #aaa;
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  white-space: nowrap;
}

.control-hint {
  font-size: 11px;
  color: #666;
}

.control-range-labels {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #666;
}

.control-panel__buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}
```

- [ ] **Step 3: Verify full project compiles**

Run: `cd /Users/logan/develop/primitive/web/frontend && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 4: Build frontend**

Run: `cd /Users/logan/develop/primitive/web/frontend && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Build full binary and manual test**

Run: `cd /Users/logan/develop/primitive && make web`
Expected: Binary builds. Manual test: run `./primitive-web`, upload an image, verify the control panel appears with all controls, configure settings, click Start, verify processing works with the configured parameters. Test playback delay by setting it to ~200ms and confirming shapes appear with visible spacing.

- [ ] **Step 6: Commit**

```bash
git add web/frontend/src/App.tsx web/frontend/src/App.css
git commit -m "feat: wire ControlPanel into App with configure state and control panel styles"
```
