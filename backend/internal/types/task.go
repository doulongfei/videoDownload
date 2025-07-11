package types

import "time"

type DownloadTask struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	Status         string    `json:"status"`
	Progress       int       `json:"progress"`
	OutputFilePath string    `json:"output_file_path"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	FileSize       int64     `json:"file_size,omitempty"`
	DownloadedSize int64     `json:"downloaded_size,omitempty"`
	DownloadSpeed  float64   `json:"download_speed,omitempty"`
	TimeRemaining  int64     `json:"time_remaining,omitempty"`
	StartTime      time.Time `json:"start_time,omitempty"`
	EndTime        time.Time `json:"end_time,omitempty"`
	AverageSpeed   float64   `json:"average_speed,omitempty"`
	TotalDuration  int64     `json:"total_duration,omitempty"`
}

type DownloadRequest struct {
	URL string `json:"url"`
}

type VideoResource struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Type        string `json:"type"`        // "m3u8", "mp4", "webm", etc.
	Quality     string `json:"quality"`     // "720p", "1080p", "unknown"
	Size        string `json:"size"`        // estimated size if available
	Duration    string `json:"duration"`    // duration if available
	Thumbnail   string `json:"thumbnail"`   // thumbnail URL if available
	Description string `json:"description"` // additional info
}

type AnalyzeRequest struct {
	URL string `json:"url"`
}

type AnalyzeResponse struct {
	Success   bool            `json:"success"`
	PageTitle string          `json:"page_title"`
	Videos    []VideoResource `json:"videos"`
	Error     string          `json:"error,omitempty"`
}