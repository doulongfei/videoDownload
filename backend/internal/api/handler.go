package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"videoDownload/internal/analyzer"
	"videoDownload/internal/downloader"
	"videoDownload/internal/types"
)

type TaskManager struct {
	tasks map[string]*types.DownloadTask
	mutex sync.RWMutex
	clients map[string][]chan *types.DownloadTask
	clientsMutex sync.RWMutex
}

var globalTaskManager = &TaskManager{
	tasks: make(map[string]*types.DownloadTask),
	clients: make(map[string][]chan *types.DownloadTask),
}

func (tm *TaskManager) AddTask(task *types.DownloadTask) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.tasks[task.ID] = task
}

func (tm *TaskManager) GetTask(id string) (*types.DownloadTask, bool) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	task, exists := tm.tasks[id]
	return task, exists
}

func (tm *TaskManager) GetAllTasks() []*types.DownloadTask {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	
	tasks := make([]*types.DownloadTask, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

func (tm *TaskManager) CompleteTask(id string, fileSize int64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	if task, exists := tm.tasks[id]; exists {
		task.Status = "completed"
		task.Progress = 100
		task.UpdatedAt = time.Now()
		task.EndTime = time.Now()
		task.FileSize = fileSize
		task.DownloadedSize = fileSize
		
		// 计算总体下载速度和持续时间
		if !task.StartTime.IsZero() {
			duration := task.EndTime.Sub(task.StartTime)
			task.TotalDuration = int64(duration.Seconds())
			
			if task.TotalDuration > 0 {
				task.AverageSpeed = float64(fileSize) / float64(task.TotalDuration)
			}
		}
		
		// 通知所有订阅的客户端
		tm.broadcastUpdate(task)
	}
}

func (tm *TaskManager) UpdateTask(id string, status string, progress int, errorMsg string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	if task, exists := tm.tasks[id]; exists {
		task.Status = status
		task.Progress = progress
		task.UpdatedAt = time.Now()
		if errorMsg != "" {
			task.ErrorMessage = errorMsg
		}
		
		// 通知所有订阅的客户端
		tm.broadcastUpdate(task)
	}
}

func (tm *TaskManager) UpdateTaskWithDetails(id string, status string, progress int, errorMsg string, downloadedSize, fileSize int64, speed float64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	
	if task, exists := tm.tasks[id]; exists {
		task.Status = status
		task.Progress = progress
		task.UpdatedAt = time.Now()
		task.DownloadedSize = downloadedSize
		task.FileSize = fileSize
		task.DownloadSpeed = speed
		
		if speed > 0 && fileSize > downloadedSize {
			remainingBytes := fileSize - downloadedSize
			task.TimeRemaining = int64(float64(remainingBytes) / speed)
		}
		
		if errorMsg != "" {
			task.ErrorMessage = errorMsg
		}
		
		// 通知所有订阅的客户端
		tm.broadcastUpdate(task)
	}
}

func (tm *TaskManager) broadcastUpdate(task *types.DownloadTask) {
	tm.clientsMutex.RLock()
	clients := tm.clients[task.ID]
	tm.clientsMutex.RUnlock()
	
	for _, ch := range clients {
		select {
		case ch <- task:
		default:
			// 客户端通道已满，跳过
		}
	}
}

func (tm *TaskManager) AddClient(taskID string, ch chan *types.DownloadTask) {
	tm.clientsMutex.Lock()
	defer tm.clientsMutex.Unlock()
	
	tm.clients[taskID] = append(tm.clients[taskID], ch)
}

func (tm *TaskManager) RemoveClient(taskID string, ch chan *types.DownloadTask) {
	tm.clientsMutex.Lock()
	defer tm.clientsMutex.Unlock()
	
	clients := tm.clients[taskID]
	for i, client := range clients {
		if client == ch {
			tm.clients[taskID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	
	if len(tm.clients[taskID]) == 0 {
		delete(tm.clients, taskID)
	}
}

func CreateDownloadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req types.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	taskID := uuid.New().String()
	outputFilename := fmt.Sprintf("video_%s.mp4", taskID[:8])
	
	task := &types.DownloadTask{
		ID:             taskID,
		URL:            req.URL,
		Status:         "pending",
		Progress:       0,
		OutputFilePath: outputFilename,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		StartTime:      time.Now(), // 记录开始时间
	}

	globalTaskManager.AddTask(task)

	go executeDownload(taskID, req.URL, outputFilename)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func AnalyzeVideoResourcesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req types.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// 创建视频分析器
	videoAnalyzer := analyzer.NewVideoAnalyzer()
	
	// 分析视频资源
	result, err := videoAnalyzer.AnalyzeURL(req.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Analysis failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func GetAllStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tasks := globalTaskManager.GetAllTasks()
	json.NewEncoder(w).Encode(tasks)
}

func GetTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	taskID := vars["id"]

	task, exists := globalTaskManager.GetTask(taskID)
	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(task)
}

func executeDownload(taskID, url, outputFilename string) {
	globalTaskManager.UpdateTask(taskID, "downloading", 0, "")

	startTime := time.Now()
	var lastUpdateTime time.Time
	var lastDownloadedSize int64

	progressCallback := func(current, total int) {
		if total > 0 {
			progress := int(float64(current) / float64(total) * 100)
			
			// 使用分片数量计算估算的文件大小（每个分片约 1MB）
			estimatedTotalSize := int64(total * 1024 * 1024)
			downloadedSize := int64(current * 1024 * 1024)
			
			// 计算下载速度
			currentTime := time.Now()
			var speed float64
			if !lastUpdateTime.IsZero() && currentTime.Sub(lastUpdateTime) > time.Second {
				timeDiff := currentTime.Sub(lastUpdateTime).Seconds()
				sizeDiff := downloadedSize - lastDownloadedSize
				speed = float64(sizeDiff) / timeDiff
				lastUpdateTime = currentTime
				lastDownloadedSize = downloadedSize
			} else if lastUpdateTime.IsZero() {
				lastUpdateTime = currentTime
				lastDownloadedSize = downloadedSize
				speed = float64(downloadedSize) / time.Since(startTime).Seconds()
			}
			
			globalTaskManager.UpdateTaskWithDetails(taskID, "downloading", progress, "", downloadedSize, estimatedTotalSize, speed)
		}
	}

	err := downloadWithProgress(url, outputFilename, progressCallback)
	
	if err != nil {
		globalTaskManager.UpdateTask(taskID, "error", 0, err.Error())
	} else {
		// 获取下载完成后的文件大小
		fileInfo, err := os.Stat(outputFilename)
		var fileSize int64
		if err == nil {
			fileSize = fileInfo.Size()
		}
		
		globalTaskManager.CompleteTask(taskID, fileSize)
	}
}

func downloadWithProgress(url, outputFilename string, progressCallback func(int, int)) error {
	return downloader.DownloadM3U8(url, outputFilename, progressCallback)
}

// SSE 处理函数
func TaskProgressSSEHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]
	
	// 检查任务是否存在
	if _, exists := globalTaskManager.GetTask(taskID); !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	
	// 设置 SSE 头部
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")
	
	// 创建客户端通道
	clientChan := make(chan *types.DownloadTask, 10)
	globalTaskManager.AddClient(taskID, clientChan)
	
	// 发送当前任务状态
	if task, exists := globalTaskManager.GetTask(taskID); exists {
		taskJSON, _ := json.Marshal(task)
		fmt.Fprintf(w, "data: %s\n\n", taskJSON)
		w.(http.Flusher).Flush()
	}
	
	// 监听客户端断开连接
	notify := r.Context().Done()
	
	for {
		select {
		case task := <-clientChan:
			taskJSON, err := json.Marshal(task)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", taskJSON)
			w.(http.Flusher).Flush()
			
			// 如果任务完成或出错，关闭连接
			if task.Status == "completed" || task.Status == "error" {
				globalTaskManager.RemoveClient(taskID, clientChan)
				close(clientChan)
				return
			}
		case <-notify:
			// 客户端断开连接
			globalTaskManager.RemoveClient(taskID, clientChan)
			close(clientChan)
			return
		}
	}
}