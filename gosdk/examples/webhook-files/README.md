# Webhook File Upload Example

This example demonstrates how to use the Discord webhook client to send files.

## Features Demonstrated

- Single file upload
- Multiple file uploads (up to 10 files)
- Different file types (text, JSON, code, logs)
- Combining files with rich embeds
- File size validation

## Setup

1. Set your Discord webhook URL:
```bash
export DISCORD_WEBHOOK="https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
```

2. Run the example:
```bash
go run main.go
```

## What it does

### Example 1: Single Text File
Sends a simple text file with a message.

### Example 2: Multiple Files
Uploads three files simultaneously:
- `report.md` - Markdown file
- `metadata.json` - JSON file
- `example.go` - Go source code file

### Example 3: Log File with Embed
Sends an application log file with a rich embed containing:
- Summary of log entries
- Issue highlights
- Metadata (lines, time range, etc.)

## File Upload Limits

- **Maximum file size**: 25 MB per file
- **Maximum total size**: 8 MB (free tier) or 100 MB (Nitro)
- **Maximum files**: 10 files per message

## Real-World Use Cases

- **Build artifacts**: Upload build logs, test results
- **Monitoring**: Send application logs with errors
- **Reports**: Generate and share reports (CSV, PDF, etc.)
- **Backups**: Upload configuration files or small backups
- **Screenshots**: Share error screenshots from automated tests

## Reading Files from Disk

To upload an actual file from disk:

```go
file, err := os.Open("path/to/file.txt")
if err != nil {
    log.Fatal(err)
}
defer file.Close()

stat, _ := file.Stat()

files := []webhook.FileAttachment{
    {
        Name:        stat.Name(),
        ContentType: "text/plain",
        Reader:      file,
        Size:        stat.Size(),
    },
}

err = client.SendWithFiles(ctx, msg, files)
```

## Error Handling

The client automatically handles:
- File size validation
- Retry logic for failed uploads
- Rate limiting (429 responses)
- Network errors with backoff

## Notes

- Files are sent using multipart/form-data encoding
- Content-Type is optional (defaults to application/octet-stream)
- File size is optional but recommended for validation
- All validations happen before the upload starts
