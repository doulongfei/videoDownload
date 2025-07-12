# Go Video Downloader

一个基于Go语言开发的智能视频下载器，具有Web界面和智能视频资源检测功能。

## 📋 功能特性

- **智能视频检测**: 自动分析网页内容，识别并提取视频资源
- **多格式支持**: 支持M3U8、MP4、WebM等多种视频格式
- **实时进度监控**: 基于Server-Sent Events (SSE)的实时下载进度更新
- **并发下载**: 支持多任务并发下载，自动重试机制
- **现代化Web界面**: 使用Alpine.js和Tailwind CSS构建的响应式界面
- **RESTful API**: 完整的REST API支持，便于集成

## 🏗️ 项目架构

### 后端 (Go)
- **主服务**: `backend/cmd/main.go` - HTTP服务器入口，CORS中间件配置
- **API处理**: `backend/internal/api/handler.go` - REST API处理器和任务管理
- **M3U8下载**: `backend/internal/downloader/m3u8.go` - M3U8解析和分段下载逻辑
- **视频分析**: `backend/internal/analyzer/video_analyzer.go` - 网页视频资源智能检测
- **数据结构**: `backend/internal/types/task.go` - 下载任务和视频资源数据结构

### 前端 (HTML + Alpine.js)
- **单页应用**: `frontend/index.html` - 完整的Web界面
- **实时更新**: 自动连接SSE流，实时显示下载进度
- **响应式设计**: 支持桌面和移动设备

## 🚀 快速开始

### 环境要求

- **Go 1.24.4+**
- **FFmpeg** (用于视频合并)

### 安装与运行

1. **克隆项目**
```bash
git clone https://github.com/doulongfei/videoDownload.git
cd videoDownload
```

2. **安装依赖**
```bash
cd backend
go mod download
```

3. **编译运行**
```bash
# 编译
go build -o server ./cmd

# 运行服务器 (默认端口5000)
./server

# 或直接运行
go run ./cmd/main.go
```

4. **访问界面**
打开浏览器访问: `http://localhost:5000`

## 📖 API文档

### 核心端点

| 方法 | 端点 | 描述 |
|------|------|------|
| `POST` | `/api/analyze` | 分析网页提取视频资源 |
| `POST` | `/api/download` | 创建下载任务 |
| `GET` | `/api/status` | 获取所有任务状态 |
| `GET` | `/api/status/{id}` | 获取指定任务状态 |
| `GET` | `/api/progress/{id}` | SSE实时进度流 |
| `GET` | `/api/health` | 健康检查 |

### 使用示例

**分析视频资源**
```bash
curl -X POST http://localhost:5000/api/analyze \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/video-page"}'
```

**创建下载任务**
```bash
curl -X POST http://localhost:5000/api/download \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/video.m3u8", "filename": "my-video"}'
```

## 🔧 技术特性

### 智能视频检测
- 解析HTML内容识别`<video>`、`<source>`标签
- 从JavaScript代码中提取视频链接
- 支持相对URL自动转换为绝对URL
- 自动检测视频质量和元数据

### 并发下载模型
- 基于Goroutine的异步执行
- 信号量限制并发数(最大10个分段同时下载)
- 线程安全的任务管理(RWMutex)
- 自动重试机制(最多3次尝试)

### 实时进度更新
- Server-Sent Events (SSE)实时流
- 每个下载任务独立的进度端点
- 前端自动连接活跃下载的SSE流

## 📁 项目结构

```
videoDownload/
├── backend/                 # Go后端
│   ├── cmd/
│   │   └── main.go         # 服务器入口
│   ├── internal/
│   │   ├── api/
│   │   │   └── handler.go  # API处理器
│   │   ├── analyzer/
│   │   │   └── video_analyzer.go  # 视频分析器
│   │   ├── downloader/
│   │   │   └── m3u8.go     # M3U8下载器
│   │   └── types/
│   │       └── task.go     # 数据结构
│   ├── go.mod
│   └── go.sum
├── frontend/
│   └── index.html          # Web界面
├── CLAUDE.md               # 开发指南
└── README.md
```

## 🔒 安全说明

本项目仅用于合法的视频下载用途。使用者需要：
- 确保拥有下载内容的合法权限
- 遵守相关版权法律法规
- 不得用于侵犯他人版权的行为

## 🤝 贡献

欢迎提交Issue和Pull Request来改进这个项目！

## 📄 许可证

本项目采用MIT许可证 - 详见LICENSE文件。

## 🙏 致谢

- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP路由
- [Alpine.js](https://alpinejs.dev/) - 前端框架
- [Tailwind CSS](https://tailwindcss.com/) - CSS框架
- [UUID](https://github.com/google/uuid) - UUID生成