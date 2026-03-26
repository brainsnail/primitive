# Primitive Web UI — Design Spec

## Overview

A web UI for the primitive image-to-geometric-art tool. Users upload an image and watch shapes appear in real time as the algorithm runs. Starts as a minimal personal tool, architected to grow into a full dashboard and eventually be hosted for others.

## Goals

- **MVP scope:** Upload an image, watch shapes render progressively, stop early, download result
- **Designed for growth:** Architecture supports adding controls, stats, pause/resume, and multi-format downloads later
- **Single binary deployment:** Go backend embeds the React frontend — one artifact to build and deploy

## Architecture

**Embedded SPA approach.** A Go HTTP server serves the React app as static files via `go:embed`. The server exposes a REST endpoint for job creation and a WebSocket endpoint for streaming shape updates.

### Components

- **HTTP Server** — serves embedded React static files at `/`, handles API routes
- **Job Manager** — wraps `primitive.Model`, manages job lifecycle (create, run, stop), runs `model.Step()` on a goroutine, pushes SVG elements to connected WebSocket clients
- **React SPA** — upload dropzone, live SVG canvas, toolbar (stop/download/new)

### Data Flow

1. User drops image on upload zone
2. Frontend POSTs image + parameters to `POST /api/jobs` → receives `{ "id": "abc123" }`
3. Frontend opens `WS /api/ws/{id}`
4. Server sends `started` message with SVG dimensions and background color
5. Server runs `model.Step()` in a loop, sends a `shape` message after each step
6. Client appends each SVG element to the canvas
7. On completion, server sends `done` message; on early stop, client sends `stop` message

## API

### REST

**`POST /api/jobs`** — multipart form upload
- Fields: `image` (file), `mode` (int, default 1), `count` (int, default 100), `alpha` (int, default 128)
- Response: `{ "id": "<job-id>" }`

### WebSocket Protocol (`/api/ws/{id}`)

Server → Client messages:

```json
{ "type": "started", "width": 400, "height": 300, "background": "#8a6b4f", "count": 100 }
{ "type": "shape", "svg": "<ellipse cx=\"100\" ... fill=\"#3a2b1c\" fill-opacity=\"0.5\"/>", "score": 0.1823, "index": 1 }
{ "type": "done", "totalShapes": 100, "finalScore": 0.0812 }
```

Client → Server messages:

```json
{ "type": "stop" }
```

The `started` message provides everything needed to initialize the SVG element. Each `shape` message contains a ready-to-append SVG element. The `score` field is included for the eventual stats panel but ignored in the MVP.

## Frontend

### UI States

1. **Idle** — Drag-and-drop upload zone. Accepts PNG/JPG. On drop, creates job with default parameters and opens WebSocket.
2. **Processing** — SVG canvas fills with shapes in real time. Shape counter overlay (`N / total`). Stop button sends `{"type": "stop"}` over WebSocket.
3. **Complete** — Canvas stays visible. Download buttons (SVG, PNG). "New" button resets to idle. PNG download is client-side: render the SVG to a `<canvas>` element and export as a PNG blob.

### React Components

- `App` — top-level state machine (idle → processing → done)
- `UploadZone` — drag-and-drop file input, triggers job creation
- `Canvas` — renders growing SVG from WebSocket messages
- `Toolbar` — contextual buttons (stop during processing, download/new when done)
- `useJob` hook — manages WebSocket connection, job state, message handling

### Layout

Single centered column. This intentionally leaves room for a sidebar controls panel in future iterations without rearchitecting the page.

## Project Structure

```
web/
├── server/
│   ├── server.go       # HTTP handlers, WebSocket, static file serving
│   └── job.go          # Job manager — wraps primitive.Model, manages lifecycle
├── frontend/
│   ├── package.json
│   ├── vite.config.ts  # Proxy /api → Go server in dev
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   │   ├── UploadZone.tsx
│   │   │   ├── Canvas.tsx
│   │   │   └── Toolbar.tsx
│   │   └── hooks/
│   │       └── useJob.ts
│   └── dist/            # Build output, embedded by Go
└── cmd/
    └── web/
        └── main.go      # Entry point — starts server, embeds frontend dist/
```

The existing CLI (`main.go` at root) remains untouched. `cmd/web/main.go` is the new entry point for the web version.

## Build

- **Dev:** Run `go run ./web/cmd/web` and `cd web/frontend && npm run dev` (Vite proxies `/api` to the Go server)
- **Prod:** `cd web/frontend && npm run build` then `go build -o primitive-web ./web/cmd/web` — produces a single binary with the frontend embedded

## Dependencies

### Go (new)
- `github.com/coder/websocket` — WebSocket support (actively maintained successor to nhooyr.io/websocket)

### Frontend
- React 19
- Vite (build tool)
- TypeScript

## Future Growth Path

The architecture supports incremental additions without restructuring:

- **Controls panel:** Add a sidebar component with shape type, count, alpha inputs. Pass as parameters to `POST /api/jobs`.
- **Live stats:** The `score` field in shape messages feeds a stats display. Add shapes/sec calculation client-side.
- **Pause/resume:** Extend the WebSocket protocol with `pause`/`resume` client messages. Job manager pauses the `model.Step()` loop.
- **Job history:** Add persistence (SQLite or filesystem) to the job manager. List past jobs, re-download results.
- **Multi-user hosting:** Add concurrency limits to the job manager, queue excess jobs, add basic rate limiting.
