# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based video downloader with web interface and intelligent video resource detection. It consists of:
- **Backend**: Go HTTP server with REST API for video downloading and resource analysis
- **Frontend**: Single-page HTML application with Alpine.js for UI
- **Video Analyzer**: Intelligent detection of video resources from web pages

## Architecture

### Backend Structure
- `backend/cmd/main.go` - Entry point, HTTP server setup with CORS middleware
- `backend/internal/api/handler.go` - REST API handlers and task management
- `backend/internal/downloader/m3u8.go` - M3U8 parsing and segment downloading logic
- `backend/internal/analyzer/video_analyzer.go` - Web page video resource detection
- `backend/internal/types/task.go` - Data structures for download tasks and video resources

### Key Components
1. **TaskManager**: Thread-safe in-memory storage for download tasks with progress tracking
2. **M3U8 Downloader**: Concurrent segment downloading with retry logic and FFmpeg merging
3. **Video Analyzer**: Intelligent parsing of web pages to extract video resources
4. **REST API**: Endpoints for creating downloads, analyzing pages, checking status, and retrieving tasks
5. **Web UI**: Real-time progress monitoring with automatic updates and video resource selection

## Development Commands

### Building and Running
```bash
# Build the Go binary
cd backend && go build -o server ./cmd

# Run the server (serves on port 5000)
cd backend && go run ./cmd/main.go

# Or run the pre-built binary
cd backend && ./server
```

### Dependencies
- **Go 1.24.4** (see go.mod)
- **FFmpeg** (required for video merging)
- **Runtime dependencies**: 
  - github.com/google/uuid
  - github.com/gorilla/mux
  - golang.org/x/net/html (for web page parsing)

### API Endpoints
- `POST /api/analyze` - Analyze web page for video resources
- `POST /api/download` - Create download task
- `GET /api/status` - Get all tasks
- `GET /api/status/{id}` - Get specific task
- `GET /api/progress/{id}` - SSE real-time progress updates
- `GET /api/health` - Health check

## Technical Notes

### Video Resource Detection
- Analyzes HTML content to detect video resources (MP4, M3U8, WebM, etc.)
- Extracts video information from `<video>` tags, `<source>` tags, and JavaScript content
- Supports quality detection and video metadata extraction
- Handles relative URLs and resolves them to absolute URLs

### Real-time Progress Updates
- Uses Server-Sent Events (SSE) for real-time progress streaming
- Each download task has its own SSE endpoint at `/api/progress/{taskId}`
- Progress is updated as each M3U8 segment is downloaded
- Frontend automatically connects to SSE streams for active downloads

### Concurrency Model
- Uses goroutines for async download execution
- Semaphore-based rate limiting (max 10 concurrent segment downloads)
- Thread-safe task management with RWMutex

### File Organization
- Downloaded videos saved as `video_{taskId}.mp4` in backend directory
- Temporary segment files stored in system temp directory during processing
- Frontend served as static files from `/frontend/` directory

### Error Handling
- Automatic retry logic for failed segment downloads (up to 3 attempts)
- Comprehensive error reporting through task status updates
- HTTP timeout handling (30 seconds per segment)