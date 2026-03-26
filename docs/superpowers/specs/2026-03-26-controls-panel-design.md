# Controls Panel — Design Spec

## Overview

Add a configuration screen between image upload and processing. Users select shape type, count, alpha, input/output size, and playback speed before starting. Playback delay is client-side only — the server runs at full speed and the frontend buffers messages.

## UI Flow

**idle** → (drop image) → **configure** → (click Start) → **processing** → **done**

The `UploadZone` now captures the file and transitions to the configure state instead of immediately starting a job.

## Configure Screen

Shows after image upload, before processing begins.

**Layout:** Thumbnail preview of the uploaded image on the left, settings form on the right. On narrow screens, thumbnail stacks above the form.

**Controls:**

| Control | Type | Default | Range/Options |
|---------|------|---------|---------------|
| Shape type | Dropdown | Triangle (1) | Combo (0), Triangle (1), Rectangle (2), Ellipse (3), Circle (4), Rotated Rect (5), Bezier (6), Rotated Ellipse (7), Polygon (8) |
| Shape count | Number input | 100 | 1–1000 |
| Alpha | Slider + number | 128 | 1–255, with "Auto" checkbox (sends 0) |
| Input size | Number input | 256 | 64–1024 (processing resolution — smaller = faster) |
| Output size | Number input | 1024 | 256–4096 (render resolution) |
| Playback delay | Slider | 0ms | 0–500ms (label: "Instant" at 0, "Slow" at 500) |

**Buttons:**
- **Start** — submits config, transitions to processing
- **Back** — returns to upload screen (discards selected image)

## Backend Changes

### JobConfig

Add `InputSize` and `OutputSize` fields:

```go
type JobConfig struct {
    Mode       int `json:"mode"`
    Count      int `json:"count"`
    Alpha      int `json:"alpha"`
    InputSize  int `json:"inputSize"`
    OutputSize int `json:"outputSize"`
}
```

### handleCreateJob

Parse two new form fields:
- `inputSize` — int, default 256, clamped to 64–1024
- `outputSize` — int, default 1024, clamped to 256–4096

### NewJob

Use `config.InputSize` and `config.OutputSize` instead of hardcoded 256 and 1024.

## Frontend Changes

### New: ControlPanel component

`src/components/ControlPanel.tsx`

Props:
- `file: File` — the uploaded image (for thumbnail preview)
- `onStart: (config: JobParams) => void` — called with settings when user clicks Start
- `onBack: () => void` — returns to upload screen

Creates an object URL from the file for the thumbnail preview. Manages local form state for all controls. On submit, passes a `JobParams` object to `onStart`.

### JobParams type

```ts
interface JobParams {
    mode: number;
    count: number;
    alpha: number;
    inputSize: number;
    outputSize: number;
}
```

### useJob changes

- `start(file, params)` — accepts `JobParams` and includes all fields in the multipart POST form data
- New `delay` property — controlled externally, determines ms between shape renders
- Message buffering: incoming `shape` messages are pushed to a queue. A `setInterval` timer drains one shape from the queue per `delay` ms. When delay is 0, shapes are applied immediately (no buffering). The delay can be changed during processing.

### App.tsx changes

- New state: `file: File | null` — holds the uploaded file between upload and configure states
- `UploadZone.onFile` now sets the file and transitions to configure (instead of calling `job.start`)
- When `file` is set and `job.state === "idle"`, render `ControlPanel`
- `ControlPanel.onStart` calls `job.start(file, params)` and clears the file
- `ControlPanel.onBack` clears the file (back to upload)
- Playback delay slider value is managed in App state and passed to `useJob`

### No changes to

- WebSocket protocol (no new message types)
- Canvas component
- Toolbar component (stop/download/new still work as before)
- server.go WebSocket handler
