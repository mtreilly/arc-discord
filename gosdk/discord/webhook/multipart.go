package webhook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/yourorg/arc-discord/gosdk/discord/types"
)

const (
	// MaxFileSize is the maximum size for a single file.
	// Discord defaults to 25MB for most servers; higher tiers can override via configuration later.
	MaxFileSize = 25 * 1024 * 1024

	// MaxTotalSize is the maximum total size for all files in a single message.
	// Kept in sync with MaxFileSize to avoid rejecting valid uploads (server boosts may increase this further).
	MaxTotalSize = 25 * 1024 * 1024

	// MaxFiles is the maximum number of files per message
	MaxFiles = 10
)

// FileAttachment represents a file to be uploaded via webhook
type FileAttachment struct {
	// Name is the filename (e.g., "image.png")
	Name string

	// ContentType is the MIME type (e.g., "image/png")
	// If empty, defaults to "application/octet-stream"
	ContentType string

	// Reader provides the file content
	Reader io.Reader

	// Size is the file size in bytes (optional, for validation)
	Size int64
}

// Validate checks if the file attachment is valid
func (f *FileAttachment) Validate() error {
	if f.Name == "" {
		return &types.ValidationError{
			Field:   "name",
			Message: "filename is required",
		}
	}

	if f.Reader == nil {
		return &types.ValidationError{
			Field:   "reader",
			Message: "file reader is required",
		}
	}

	if f.Size > MaxFileSize {
		return &types.ValidationError{
			Field:   "size",
			Message: fmt.Sprintf("file size %d exceeds maximum %d bytes (25MB)", f.Size, MaxFileSize),
		}
	}

	return nil
}

// resolvedSize returns the declared or detected size of the attachment.
// The second return value indicates whether the size is known.
func (f *FileAttachment) resolvedSize() (int64, bool, error) {
	if f.Size > 0 {
		return f.Size, true, nil
	}

	if f.Reader == nil {
		return 0, false, nil
	}

	// Prefer interfaces with Len() to avoid touching the underlying reader state.
	type lenInt interface{ Len() int }
	type lenInt64 interface{ Len() int64 }

	if l, ok := f.Reader.(lenInt); ok && l.Len() >= 0 {
		return int64(l.Len()), true, nil
	}
	if l, ok := f.Reader.(lenInt64); ok && l.Len() >= 0 {
		return l.Len(), true, nil
	}

	// Fall back to io.Seeker if available.
	if seeker, ok := f.Reader.(io.Seeker); ok {
		current, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, false, fmt.Errorf("failed to read current position for %s: %w", f.Name, err)
		}

		end, err := seeker.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, false, fmt.Errorf("failed to determine size for %s: %w", f.Name, err)
		}

		if _, err := seeker.Seek(current, io.SeekStart); err != nil {
			return 0, false, fmt.Errorf("failed to restore reader position for %s: %w", f.Name, err)
		}

		return end - current, true, nil
	}

	return 0, false, nil
}

// SendWithFiles sends a webhook message with file attachments
func (c *Client) SendWithFiles(ctx context.Context, msg *types.WebhookMessage, files []FileAttachment) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid webhook message: %w", err)
	}

	if len(files) == 0 {
		return &types.ValidationError{
			Field:   "files",
			Message: "at least one file is required (use Send for messages without files)",
		}
	}

	if len(files) > MaxFiles {
		return &types.ValidationError{
			Field:   "files",
			Message: fmt.Sprintf("too many files: %d (maximum %d)", len(files), MaxFiles),
		}
	}

	// Validate all files and accumulate known sizes
	var totalSize int64
	for i := range files {
		if err := (&files[i]).Validate(); err != nil {
			return fmt.Errorf("file %d validation failed: %w", i, err)
		}

		size, known, err := files[i].resolvedSize()
		if err != nil {
			return fmt.Errorf("file %d size detection failed: %w", i, err)
		}

		if known {
			if size > MaxFileSize {
				return &types.ValidationError{
					Field:   "files",
					Message: fmt.Sprintf("file %s exceeds maximum %d bytes", files[i].Name, MaxFileSize),
				}
			}
			totalSize += size
		}
	}

	if MaxTotalSize > 0 && totalSize > MaxTotalSize {
		return &types.ValidationError{
			Field:   "files",
			Message: fmt.Sprintf("total file size %d exceeds maximum %d bytes", totalSize, MaxTotalSize),
		}
	}

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add JSON payload
	if err := c.writeJSONPayload(writer, msg); err != nil {
		return fmt.Errorf("failed to write JSON payload: %w", err)
	}

	// Add files
	counter := &uploadCounter{limit: MaxTotalSize}
	for i, file := range files {
		if err := c.writeFile(writer, i, file, counter); err != nil {
			return fmt.Errorf("failed to write file %d: %w", i, err)
		}
	}

	// Close multipart writer
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Build URL with thread_id query parameter if specified
	url := c.buildURLWithThreadID(c.webhookURL, msg.ThreadID)

	// Send with retry
	return c.sendMultipartWithRetry(ctx, body.Bytes(), writer.FormDataContentType(), url)
}

// writeJSONPayload writes the webhook message as JSON to the multipart form
func (c *Client) writeJSONPayload(writer *multipart.Writer, msg *types.WebhookMessage) error {
	// Create form field for JSON payload
	part, err := writer.CreateFormField("payload_json")
	if err != nil {
		return err
	}

	// Marshal message to JSON
	data, err := marshalJSON(msg)
	if err != nil {
		return err
	}

	// Write JSON to form field
	_, err = part.Write(data)
	return err
}

// writeFile writes a file attachment to the multipart form
func (c *Client) writeFile(writer *multipart.Writer, index int, file FileAttachment, counter *uploadCounter) error {
	// Create form file with unique field name
	fieldName := fmt.Sprintf("file%d", index)

	// Set content type if provided
	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Create part with custom headers
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, file.Name),
	}
	h["Content-Type"] = []string{contentType}

	part, err := writer.CreatePart(h)
	if err != nil {
		return err
	}

	// Copy file content to part
	reader := newCountingReader(file.Reader, file.Name, MaxFileSize, counter)
	_, err = io.Copy(part, reader)
	return err
}

// sendMultipartWithRetry sends a multipart request with retry logic
func (c *Client) sendMultipartWithRetry(ctx context.Context, body []byte, contentType, url string) error {
	var lastErr error
	backoff := c.timeout / 30 // Start with ~1 second
	route := c.buildRoute("POST", url)

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-waitWithBackoff(backoff):
				backoff *= 2
			}
		}

		// Rate limiting
		if err := c.waitForRateLimit(ctx, route); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", contentType)
		req.Header.Set("User-Agent", "DiscordWebhook/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &types.NetworkError{Op: "request", Err: err}
			continue
		}

		// Update rate limiter
		if c.rateLimiter != nil {
			c.rateLimiter.Update(route, resp.Header)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			c.recordStrategyOutcome(route, false)
			return nil
		}

		// Handle error response (reuse existing logic)
		apiErr := c.parseErrorResponse(resp)
		resp.Body.Close()

		// Handle rate limiting
		if resp.StatusCode == 429 {
			c.logger.Warn("rate limit hit",
				"route", route,
				"retry_after", apiErr.RetryAfter,
				"attempt", attempt+1,
				"method", "POST (multipart)",
			)
			c.recordStrategyOutcome(route, true)

			if apiErr.RetryAfter > 0 {
				backoff = backoffFromSeconds(apiErr.RetryAfter)
			}
			lastErr = apiErr
			continue
		}

		// Don't retry client errors (except rate limits)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return apiErr
		}

		// Retry server errors
		lastErr = apiErr
	}

	if lastErr != nil {
		return fmt.Errorf("multipart request failed after %d attempts: %w", c.maxRetries+1, lastErr)
	}

	return fmt.Errorf("multipart request failed after %d attempts", c.maxRetries+1)
}

type uploadCounter struct {
	limit int64
	read  int64
}

type countingReader struct {
	reader       io.Reader
	fileName     string
	perFileLimit int64
	fileRead     int64
	total        *uploadCounter
}

func newCountingReader(r io.Reader, name string, perFileLimit int64, total *uploadCounter) *countingReader {
	return &countingReader{
		reader:       r,
		fileName:     name,
		perFileLimit: perFileLimit,
		total:        total,
	}
}

func (cr *countingReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if cr.perFileLimit > 0 {
		remaining := cr.perFileLimit - cr.fileRead
		if remaining <= 0 {
			return 0, cr.perFileExceededErr()
		}
		if int64(len(p)) > remaining {
			p = p[:remaining]
		}
	}

	if cr.total != nil && cr.total.limit > 0 {
		remaining := cr.total.limit - cr.total.read
		if remaining <= 0 {
			return 0, cr.totalExceededErr()
		}
		if int64(len(p)) > remaining {
			p = p[:remaining]
		}
	}

	n, err := cr.reader.Read(p)
	if n > 0 {
		cr.fileRead += int64(n)
		if cr.total != nil {
			cr.total.read += int64(n)
		}
	}
	return n, err
}

func (cr *countingReader) perFileExceededErr() error {
	return &types.ValidationError{
		Field:   "files",
		Message: fmt.Sprintf("file %s exceeds maximum size of %d bytes", cr.fileName, cr.perFileLimit),
	}
}

func (cr *countingReader) totalExceededErr() error {
	return &types.ValidationError{
		Field:   "files",
		Message: fmt.Sprintf("total attachment size exceeds maximum %d bytes", cr.total.limit),
	}
}
