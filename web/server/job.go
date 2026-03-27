// web/server/job.go
package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	"runtime"
	"sync"

	"github.com/fogleman/primitive/primitive"
	"github.com/nfnt/resize"
)

type JobStatus string

const (
	StatusPending JobStatus = "pending"
	StatusRunning JobStatus = "running"
	StatusDone    JobStatus = "done"
	StatusStopped JobStatus = "stopped"
)

// ShapeResult holds the SVG element and score for a single shape.
type ShapeResult struct {
	SVG   string  `json:"svg"`
	Score float64 `json:"score"`
	Index int     `json:"index"`
}

// JobConfig holds the parameters for a primitive job.
type JobConfig struct {
	Mode       int `json:"mode"`
	Count      int `json:"count"`
	Alpha      int `json:"alpha"`
	InputSize  int `json:"inputSize"`
	OutputSize int `json:"outputSize"`
}

// JobInfo holds the initial dimensions and background, sent when the WebSocket connects.
type JobInfo struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Background string `json:"background"`
}

// Job wraps a primitive.Model and manages its lifecycle.
type Job struct {
	ID     string
	Status JobStatus
	Config JobConfig
	Info   JobInfo

	model  *primitive.Model
	shapes []ShapeResult
	mu     sync.Mutex
	stopCh chan struct{}
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

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

// Run executes the primitive algorithm, sending each shape result to the channel.
// The channel is closed when the job completes or is stopped.
func (j *Job) Run(results chan<- ShapeResult) {
	j.mu.Lock()
	j.Status = StatusRunning
	j.mu.Unlock()

	defer func() {
		close(results)
	}()

	for i := 0; i < j.Config.Count; i++ {
		select {
		case <-j.stopCh:
			j.mu.Lock()
			j.Status = StatusStopped
			j.mu.Unlock()
			return
		default:
		}

		j.model.Step(primitive.ShapeType(j.Config.Mode), j.Config.Alpha, 0)

		// Extract the SVG element for the shape just added.
		idx := len(j.model.Shapes) - 1
		shape := j.model.Shapes[idx]
		c := j.model.Colors[idx]
		attrs := fmt.Sprintf(`fill="#%02x%02x%02x" fill-opacity="%f"`, c.R, c.G, c.B, float64(c.A)/255)
		svgElem := shape.SVG(attrs)

		result := ShapeResult{
			SVG:   svgElem,
			Score: j.model.Score,
			Index: i + 1,
		}

		j.mu.Lock()
		j.shapes = append(j.shapes, result)
		j.mu.Unlock()

		results <- result
	}

	j.mu.Lock()
	j.Status = StatusDone
	j.mu.Unlock()
}

// Stop signals the job to stop after the current shape completes.
func (j *Job) Stop() {
	select {
	case <-j.stopCh:
		// already stopped
	default:
		close(j.stopCh)
	}
}

// SVG returns the full SVG document for the current state.
func (j *Job) SVG() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.model.SVG()
}
