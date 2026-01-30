package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

func TestFileAttachment_Validate(t *testing.T) {
	tests := []struct {
		name    string
		file    FileAttachment
		wantErr bool
	}{
		{
			name: "valid file",
			file: FileAttachment{
				Name:        "test.txt",
				ContentType: "text/plain",
				Reader:      strings.NewReader("test content"),
				Size:        12,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			file: FileAttachment{
				Reader: strings.NewReader("test"),
				Size:   4,
			},
			wantErr: true,
		},
		{
			name: "missing reader",
			file: FileAttachment{
				Name: "test.txt",
				Size: 4,
			},
			wantErr: true,
		},
		{
			name: "file too large",
			file: FileAttachment{
				Name:   "large.txt",
				Reader: strings.NewReader("test"),
				Size:   MaxFileSize + 1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.file.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("FileAttachment.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_SendWithFiles(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify content type is multipart
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data") {
			t.Errorf("Expected multipart/form-data, got %s", contentType)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		// Verify payload_json field
		payloadJSON := r.FormValue("payload_json")
		if payloadJSON == "" {
			t.Error("Missing payload_json field")
		}

		var msg types.WebhookMessage
		if err := json.Unmarshal([]byte(payloadJSON), &msg); err != nil {
			t.Errorf("Failed to unmarshal payload_json: %v", err)
		}

		if msg.Content != "test message with files" {
			t.Errorf("Expected content 'test message with files', got '%s'", msg.Content)
		}

		// Verify file uploads
		if r.MultipartForm.File["file0"] == nil {
			t.Error("Missing file0")
		}

		file, _, err := r.FormFile("file0")
		if err != nil {
			t.Errorf("Failed to get file0: %v", err)
		}
		defer file.Close()

		content, _ := io.ReadAll(file)
		if string(content) != "test file content" {
			t.Errorf("Expected 'test file content', got '%s'", string(content))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	msg := &types.WebhookMessage{
		Content: "test message with files",
	}

	files := []FileAttachment{
		{
			Name:        "test.txt",
			ContentType: "text/plain",
			Reader:      strings.NewReader("test file content"),
			Size:        17,
		},
	}

	err = client.SendWithFiles(ctx, msg, files)
	if err != nil {
		t.Errorf("SendWithFiles() error = %v", err)
	}
}

func TestClient_SendWithFiles_MultipleFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		// Verify multiple files
		if r.MultipartForm.File["file0"] == nil {
			t.Error("Missing file0")
		}
		if r.MultipartForm.File["file1"] == nil {
			t.Error("Missing file1")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	msg := &types.WebhookMessage{
		Content: "multiple files",
	}

	files := []FileAttachment{
		{
			Name:   "file1.txt",
			Reader: strings.NewReader("content 1"),
			Size:   9,
		},
		{
			Name:   "file2.txt",
			Reader: strings.NewReader("content 2"),
			Size:   9,
		},
	}

	err = client.SendWithFiles(ctx, msg, files)
	if err != nil {
		t.Errorf("SendWithFiles() error = %v", err)
	}
}

func TestClient_SendWithFiles_Validation(t *testing.T) {
	client, _ := NewClient("http://test.com")
	ctx := context.Background()

	tests := []struct {
		name    string
		msg     *types.WebhookMessage
		files   []FileAttachment
		wantErr bool
	}{
		{
			name: "no files",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			files:   []FileAttachment{},
			wantErr: true,
		},
		{
			name: "too many files",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			files: func() []FileAttachment {
				files := make([]FileAttachment, MaxFiles+1)
				for i := range files {
					files[i] = FileAttachment{
						Name:   "test.txt",
						Reader: strings.NewReader("test"),
						Size:   4,
					}
				}
				return files
			}(),
			wantErr: true,
		},
		{
			name: "total size too large (known sizes)",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			files: []FileAttachment{
				{
					Name:   "large1.txt",
					Reader: strings.NewReader("test"),
					Size:   MaxTotalSize/2 + 1,
				},
				{
					Name:   "large2.txt",
					Reader: strings.NewReader("test"),
					Size:   MaxTotalSize/2 + 1,
				},
			},
			wantErr: true,
		},
		{
			name: "per-file limit enforced without size hint",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			files: []FileAttachment{
				{
					Name:   "huge.bin",
					Reader: newFixedSizeReader(MaxFileSize + 1),
				},
			},
			wantErr: true,
		},
		{
			name: "total limit enforced without size hints",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			files: []FileAttachment{
				{
					Name:   "big1.bin",
					Reader: newFixedSizeReader(MaxTotalSize/2 + 1),
				},
				{
					Name:   "big2.bin",
					Reader: newFixedSizeReader(MaxTotalSize/2 + 1),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid file",
			msg: &types.WebhookMessage{
				Content: "test",
			},
			files: []FileAttachment{
				{
					Name:   "", // missing name
					Reader: strings.NewReader("test"),
					Size:   4,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.SendWithFiles(ctx, tt.msg, tt.files)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendWithFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type fixedSizeReader struct {
	remaining int64
}

func newFixedSizeReader(size int64) io.Reader {
	return &fixedSizeReader{remaining: size}
}

func (r *fixedSizeReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}

	if int64(len(p)) > r.remaining {
		p = p[:int(r.remaining)]
	}

	for i := range p {
		p[i] = 'a'
	}

	n := len(p)
	r.remaining -= int64(n)
	return n, nil
}

func TestClient_SendWithFiles_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(3), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	msg := &types.WebhookMessage{
		Content: "retry test",
	}

	files := []FileAttachment{
		{
			Name:   "test.txt",
			Reader: strings.NewReader("test"),
			Size:   4,
		},
	}

	err = client.SendWithFiles(ctx, msg, files)
	if err != nil {
		t.Errorf("SendWithFiles() error = %v, expected success after retries", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestClient_SendWithFiles_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":     "You are being rate limited.",
			"retry_after": 0.5,
			"global":      false,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithMaxRetries(2))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()
	msg := &types.WebhookMessage{
		Content: "rate limit test",
	}

	files := []FileAttachment{
		{
			Name:   "test.txt",
			Reader: strings.NewReader("test"),
			Size:   4,
		},
	}

	start := time.Now()
	err = client.SendWithFiles(ctx, msg, files)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error for rate limit, got nil")
	}

	// Should have retried with backoff
	if elapsed < 500*time.Millisecond {
		t.Errorf("Expected at least 500ms delay for rate limit, got %v", elapsed)
	}
}

func TestWriteJSONPayload(t *testing.T) {
	client, _ := NewClient("http://test.com")

	msg := &types.WebhookMessage{
		Content: "test",
		Embeds: []types.Embed{
			{
				Title:       "Test Embed",
				Description: "Test description",
			},
		},
	}

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)

	err := client.writeJSONPayload(writer, msg)
	if err != nil {
		t.Fatalf("writeJSONPayload() error = %v", err)
	}

	writer.Close()

	// Parse and verify
	reader := multipart.NewReader(buf, writer.Boundary())
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("Failed to read part: %v", err)
	}

	if part.FormName() != "payload_json" {
		t.Errorf("Expected form name 'payload_json', got '%s'", part.FormName())
	}

	var parsed types.WebhookMessage
	if err := json.NewDecoder(part).Decode(&parsed); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if parsed.Content != "test" {
		t.Errorf("Expected content 'test', got '%s'", parsed.Content)
	}
}

func TestWriteFile(t *testing.T) {
	client, _ := NewClient("http://test.com")

	file := FileAttachment{
		Name:        "test.txt",
		ContentType: "text/plain",
		Reader:      strings.NewReader("test content"),
	}

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	counter := &uploadCounter{limit: MaxTotalSize}

	err := client.writeFile(writer, 0, file, counter)
	if err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	writer.Close()

	// Parse and verify
	reader := multipart.NewReader(buf, writer.Boundary())
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("Failed to read part: %v", err)
	}

	if part.FormName() != "file0" {
		t.Errorf("Expected form name 'file0', got '%s'", part.FormName())
	}

	if part.FileName() != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got '%s'", part.FileName())
	}

	content, _ := io.ReadAll(part)
	if string(content) != "test content" {
		t.Errorf("Expected 'test content', got '%s'", string(content))
	}
}
