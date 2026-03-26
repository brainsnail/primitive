package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// encodePNG encodes an image to PNG bytes.
func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// createUploadRequest builds a multipart POST request with an image file.
func createUploadRequest(t *testing.T, url string, imgBytes []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "test.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(imgBytes)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestCreateJob(t *testing.T) {
	srv := NewServer()
	imgBytes := encodePNG(testImage(64, 64))
	req := createUploadRequest(t, "/api/jobs", imgBytes)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
}

func TestWebSocketStreaming(t *testing.T) {
	srv := NewServer()

	// Create a job via HTTP
	imgBytes := encodePNG(testImage(64, 64))
	req := createUploadRequest(t, "/api/jobs", imgBytes)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var createResp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(w.Result().Body).Decode(&createResp)

	// Start HTTP test server for WebSocket
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Connect via WebSocket
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws/" + createResp.ID
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.CloseNow()

	// First message should be "started"
	var started wsMessage
	if err := wsjson.Read(ctx, conn, &started); err != nil {
		t.Fatalf("read started: %v", err)
	}
	if started.Type != "started" {
		t.Fatalf("expected type 'started', got %q", started.Type)
	}
	if started.Width == 0 || started.Height == 0 {
		t.Fatal("expected non-zero dimensions")
	}
	if started.Background == "" {
		t.Fatal("expected non-empty background")
	}

	// Read shape messages — job has count=100 (default), but we just verify we get some
	shapeCount := 0
	for {
		var msg wsMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			t.Fatalf("read message: %v", err)
		}
		if msg.Type == "done" {
			if msg.TotalShapes == 0 {
				t.Fatal("expected non-zero total shapes in done message")
			}
			break
		}
		if msg.Type != "shape" {
			t.Fatalf("unexpected message type: %q", msg.Type)
		}
		if msg.SVG == "" {
			t.Fatal("expected non-empty SVG in shape message")
		}
		shapeCount++
	}

	if shapeCount == 0 {
		t.Fatal("expected at least one shape message")
	}
}

func TestWebSocketStop(t *testing.T) {
	srv := NewServer()

	// Create a job with high count so it doesn't finish before we stop
	imgBytes := encodePNG(testImage(64, 64))

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("image", "test.png")
	part.Write(imgBytes)
	writer.WriteField("count", "1000")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/jobs", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var createResp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(w.Result().Body).Decode(&createResp)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws/" + createResp.ID
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.CloseNow()

	// Read started
	var started wsMessage
	wsjson.Read(ctx, conn, &started)

	// Read a few shapes
	for i := 0; i < 3; i++ {
		var msg wsMessage
		wsjson.Read(ctx, conn, &msg)
	}

	// Send stop
	wsjson.Write(ctx, conn, wsMessage{Type: "stop"})

	// Read remaining messages until done
	for {
		var msg wsMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			break // connection closed
		}
		if msg.Type == "done" {
			if msg.TotalShapes >= 1000 {
				t.Fatalf("expected fewer than 1000 shapes after stop, got %d", msg.TotalShapes)
			}
			break
		}
	}
}
