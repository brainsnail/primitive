// web/server/job_test.go
package server

import (
	"image"
	"image/color"
	"testing"
)

// testImage creates a small solid-color image for testing.
func testImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	return img
}

func TestNewJob(t *testing.T) {
	img := testImage(64, 64)
	job := NewJob(img, JobConfig{
		Mode:  1,
		Count: 10,
		Alpha: 128,
	})
	if job == nil {
		t.Fatal("expected non-nil job")
	}
	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.Status != StatusPending {
		t.Fatalf("expected status %q, got %q", StatusPending, job.Status)
	}
	if job.Config.Count != 10 {
		t.Fatalf("expected count 10, got %d", job.Config.Count)
	}
}

func TestJobRun(t *testing.T) {
	img := testImage(64, 64)
	job := NewJob(img, JobConfig{
		Mode:  1,
		Count: 5,
		Alpha: 128,
	})

	results := make(chan ShapeResult, 10)
	done := make(chan struct{})

	go func() {
		job.Run(results)
		close(done)
	}()

	count := 0
	for range results {
		count++
	}
	<-done

	if count != 5 {
		t.Fatalf("expected 5 shapes, got %d", count)
	}
	if job.Status != StatusDone {
		t.Fatalf("expected status %q, got %q", StatusDone, job.Status)
	}
}

func TestJobStop(t *testing.T) {
	img := testImage(64, 64)
	job := NewJob(img, JobConfig{
		Mode:  1,
		Count: 1000, // large count so we can stop early
		Alpha: 128,
	})

	results := make(chan ShapeResult, 1100)
	done := make(chan struct{})

	go func() {
		job.Run(results)
		close(done)
	}()

	// Wait for at least one shape, then stop
	<-results
	job.Stop()
	<-done

	if job.Status != StatusStopped {
		t.Fatalf("expected status %q, got %q", StatusStopped, job.Status)
	}
}
