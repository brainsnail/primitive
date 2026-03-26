package server

import (
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Server handles HTTP requests for the primitive web UI.
type Server struct {
	mux      *http.ServeMux
	jobs     map[string]*Job
	jobsMu   sync.RWMutex
	frontend fs.FS // embedded frontend files, nil in tests
}

// NewServer creates a server without embedded frontend (for testing).
func NewServer() *Server {
	return NewServerWithFrontend(nil)
}

// NewServerWithFrontend creates a server that serves static files from the given FS.
func NewServerWithFrontend(frontend fs.FS) *Server {
	s := &Server{
		mux:      http.NewServeMux(),
		jobs:     make(map[string]*Job),
		frontend: frontend,
	}
	s.mux.HandleFunc("POST /api/jobs", s.handleCreateJob)
	s.mux.HandleFunc("/api/ws/", s.handleWebSocket)
	if frontend != nil {
		s.mux.Handle("/", http.FileServer(http.FS(frontend)))
	}
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (32 MB max)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		http.Error(w, "invalid image", http.StatusBadRequest)
		return
	}

	mode, _ := strconv.Atoi(r.FormValue("mode"))
	if mode == 0 {
		mode = 1
	}
	count, _ := strconv.Atoi(r.FormValue("count"))
	if count == 0 {
		count = 100
	}
	alpha, _ := strconv.Atoi(r.FormValue("alpha"))
	if alpha == 0 {
		alpha = 128
	}

	job := NewJob(img, JobConfig{
		Mode:  mode,
		Count: count,
		Alpha: alpha,
	})

	s.jobsMu.Lock()
	s.jobs[job.ID] = job
	s.jobsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": job.ID})
}

// wsMessage is the JSON structure for WebSocket messages in both directions.
type wsMessage struct {
	Type        string  `json:"type"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	Background  string  `json:"background,omitempty"`
	Count       int     `json:"count,omitempty"`
	SVG         string  `json:"svg,omitempty"`
	Score       float64 `json:"score,omitempty"`
	Index       int     `json:"index,omitempty"`
	TotalShapes int     `json:"totalShapes,omitempty"`
	FinalScore  float64 `json:"finalScore,omitempty"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path: /api/ws/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/ws/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing job ID", http.StatusBadRequest)
		return
	}
	jobID := parts[0]

	s.jobsMu.RLock()
	job, ok := s.jobs[jobID]
	s.jobsMu.RUnlock()
	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow connections from any origin (dev + prod)
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()

	// Send started message
	wsjson.Write(ctx, conn, wsMessage{
		Type:       "started",
		Width:      job.Info.Width,
		Height:     job.Info.Height,
		Background: job.Info.Background,
		Count:      job.Config.Count,
	})

	// Listen for stop messages from client
	go func() {
		for {
			var msg wsMessage
			err := wsjson.Read(ctx, conn, &msg)
			if err != nil {
				return
			}
			if msg.Type == "stop" {
				job.Stop()
				return
			}
		}
	}()

	// Run job and stream results
	results := make(chan ShapeResult, 16)
	go job.Run(results)

	var lastScore float64
	var totalShapes int
	for result := range results {
		lastScore = result.Score
		totalShapes = result.Index
		err := wsjson.Write(ctx, conn, wsMessage{
			Type:  "shape",
			SVG:   result.SVG,
			Score: result.Score,
			Index: result.Index,
		})
		if err != nil {
			job.Stop()
			return
		}
	}

	// Send done message
	wsjson.Write(ctx, conn, wsMessage{
		Type:        "done",
		TotalShapes: totalShapes,
		FinalScore:  lastScore,
	})

	conn.Close(websocket.StatusNormalClosure, "done")
}
