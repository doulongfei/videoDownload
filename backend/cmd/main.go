package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"videoDownload/internal/api"

	"github.com/gorilla/mux"
)

type HealthResponse struct {
	Status string `json:"status"`
}

// CORS中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置CORS头部
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// 处理预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 继续处理其他请求
		next.ServeHTTP(w, r)
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HealthResponse{Status: "ok"}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func main() {
	router := mux.NewRouter()

	// 应用CORS中间件到所有路由
	router.Use(corsMiddleware)

	// API路由
	router.HandleFunc("/api/health", healthHandler).Methods("GET")
	router.HandleFunc("/api/analyze", api.AnalyzeVideoResourcesHandler).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/download", api.CreateDownloadHandler).Methods("POST", "OPTIONS")
	router.HandleFunc("/api/status", api.GetAllStatusHandler).Methods("GET")
	router.HandleFunc("/api/status/{id}", api.GetTaskStatusHandler).Methods("GET")
	router.HandleFunc("/api/progress/{id}", api.TaskProgressSSEHandler).Methods("GET")

	// 静态文件服务
	frontendPath := filepath.Join("..", "frontend")
	fileServer := http.FileServer(http.Dir(frontendPath))
	router.PathPrefix("/").Handler(fileServer)

	server := &http.Server{
		Addr:    "0.0.0.0:5000",
		Handler: router,
	}

	fmt.Println("服务器启动在端口 5000...")
	fmt.Println("健康检查: http://localhost:5000/api/health")
	fmt.Println("前端页面: http://localhost:5000/")
	fmt.Println("API端点:")
	fmt.Println("  POST /api/analyze - 分析网页视频资源")
	fmt.Println("  POST /api/download - 创建下载任务")
	fmt.Println("  GET  /api/status - 获取所有任务状态")
	fmt.Println("  GET  /api/status/{id} - 获取指定任务状态")
	fmt.Println("  GET  /api/progress/{id} - SSE 实时进度推送")
	fmt.Println("CORS已启用，支持跨域请求")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}
