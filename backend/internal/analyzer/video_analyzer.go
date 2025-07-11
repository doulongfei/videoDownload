package analyzer

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/html"
	"videoDownload/internal/types"
)

type VideoAnalyzer struct {
	client *http.Client
}

func NewVideoAnalyzer() *VideoAnalyzer {
	return &VideoAnalyzer{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (va *VideoAnalyzer) AnalyzeURL(targetURL string) (*types.AnalyzeResponse, error) {
	resp, err := va.client.Get(targetURL)
	if err != nil {
		return &types.AnalyzeResponse{
			Success: false,
			Error:   fmt.Sprintf("无法访问网页: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &types.AnalyzeResponse{
			Success: false,
			Error:   fmt.Sprintf("HTTP 错误: %d", resp.StatusCode),
		}, nil
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return &types.AnalyzeResponse{
			Success: false,
			Error:   fmt.Sprintf("解析HTML失败: %v", err),
		}, nil
	}

	pageTitle := va.extractPageTitle(doc)
	videos := va.extractVideoResources(doc, targetURL)

	return &types.AnalyzeResponse{
		Success:   true,
		PageTitle: pageTitle,
		Videos:    videos,
	}, nil
}

func (va *VideoAnalyzer) extractPageTitle(doc *html.Node) string {
	var title string
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "title" {
			if node.FirstChild != nil {
				title = strings.TrimSpace(node.FirstChild.Data)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(doc)
	
	if title == "" {
		title = "未知页面"
	}
	return title
}

func (va *VideoAnalyzer) extractVideoResources(doc *html.Node, baseURL string) []types.VideoResource {
	var videos []types.VideoResource
	
	// 解析基础URL
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return videos
	}

	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "video":
				videos = append(videos, va.extractFromVideoTag(node, parsedBaseURL)...)
			case "source":
				if parent := node.Parent; parent != nil && parent.Data == "video" {
					videos = append(videos, va.extractFromSourceTag(node, parsedBaseURL)...)
				}
			case "script":
				videos = append(videos, va.extractFromScript(node, parsedBaseURL)...)
			case "a":
				videos = append(videos, va.extractFromLink(node, parsedBaseURL)...)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(doc)

	// 去重
	return va.deduplicateVideos(videos)
}

func (va *VideoAnalyzer) extractFromVideoTag(node *html.Node, baseURL *url.URL) []types.VideoResource {
	var videos []types.VideoResource
	
	var src, poster string
	for _, attr := range node.Attr {
		switch attr.Key {
		case "src":
			src = attr.Val
		case "poster":
			poster = attr.Val
		}
	}
	
	if src != "" {
		videoURL := va.resolveURL(src, baseURL)
		if videoURL != "" {
			videos = append(videos, types.VideoResource{
				ID:        uuid.New().String(),
				Title:     va.extractVideoTitle(node),
				URL:       videoURL,
				Type:      va.detectVideoType(videoURL),
				Quality:   "unknown",
				Thumbnail: va.resolveURL(poster, baseURL),
			})
		}
	}
	
	return videos
}

func (va *VideoAnalyzer) extractFromSourceTag(node *html.Node, baseURL *url.URL) []types.VideoResource {
	var videos []types.VideoResource
	
	var src, typeAttr string
	for _, attr := range node.Attr {
		switch attr.Key {
		case "src":
			src = attr.Val
		case "type":
			typeAttr = attr.Val
		}
	}
	
	if src != "" {
		videoURL := va.resolveURL(src, baseURL)
		if videoURL != "" {
			videos = append(videos, types.VideoResource{
				ID:      uuid.New().String(),
				Title:   va.extractVideoTitle(node.Parent),
				URL:     videoURL,
				Type:    va.detectVideoTypeFromMime(typeAttr),
				Quality: "unknown",
			})
		}
	}
	
	return videos
}

func (va *VideoAnalyzer) extractFromScript(node *html.Node, baseURL *url.URL) []types.VideoResource {
	var videos []types.VideoResource
	
	if node.FirstChild != nil {
		scriptContent := node.FirstChild.Data
		videos = append(videos, va.extractFromScriptContent(scriptContent, baseURL)...)
	}
	
	return videos
}

func (va *VideoAnalyzer) extractFromScriptContent(content string, baseURL *url.URL) []types.VideoResource {
	var videos []types.VideoResource
	
	// 常见的视频URL模式
	patterns := []string{
		`["']([^"']*\.m3u8[^"']*)["']`,
		`["']([^"']*\.mp4[^"']*)["']`,
		`["']([^"']*\.webm[^"']*)["']`,
		`["']([^"']*\.mov[^"']*)["']`,
		`["']([^"']*\.avi[^"']*)["']`,
		`["']([^"']*\.flv[^"']*)["']`,
		`["']([^"']*\.mkv[^"']*)["']`,
		`src:\s*["']([^"']*\.m3u8[^"']*)["']`,
		`src:\s*["']([^"']*\.mp4[^"']*)["']`,
		`url:\s*["']([^"']*\.m3u8[^"']*)["']`,
		`url:\s*["']([^"']*\.mp4[^"']*)["']`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		
		for _, match := range matches {
			if len(match) > 1 {
				videoURL := va.resolveURL(match[1], baseURL)
				if videoURL != "" && va.isValidVideoURL(videoURL) {
					videos = append(videos, types.VideoResource{
						ID:      uuid.New().String(),
						Title:   "从脚本中提取",
						URL:     videoURL,
						Type:    va.detectVideoType(videoURL),
						Quality: va.detectQuality(videoURL),
					})
				}
			}
		}
	}
	
	return videos
}

func (va *VideoAnalyzer) extractFromLink(node *html.Node, baseURL *url.URL) []types.VideoResource {
	var videos []types.VideoResource
	
	var href, title string
	for _, attr := range node.Attr {
		switch attr.Key {
		case "href":
			href = attr.Val
		case "title":
			title = attr.Val
		}
	}
	
	if href != "" && va.isVideoLink(href) {
		videoURL := va.resolveURL(href, baseURL)
		if videoURL != "" {
			if title == "" {
				title = va.extractLinkText(node)
			}
			videos = append(videos, types.VideoResource{
				ID:      uuid.New().String(),
				Title:   title,
				URL:     videoURL,
				Type:    va.detectVideoType(videoURL),
				Quality: va.detectQuality(videoURL),
			})
		}
	}
	
	return videos
}

func (va *VideoAnalyzer) extractVideoTitle(node *html.Node) string {
	if node == nil {
		return "未知视频"
	}
	
	// 尝试从属性中获取标题
	for _, attr := range node.Attr {
		if attr.Key == "title" || attr.Key == "alt" {
			return attr.Val
		}
	}
	
	// 尝试从文本内容中获取
	return va.extractTextContent(node)
}

func (va *VideoAnalyzer) extractLinkText(node *html.Node) string {
	if node == nil {
		return "未知链接"
	}
	return va.extractTextContent(node)
}

func (va *VideoAnalyzer) extractTextContent(node *html.Node) string {
	var text strings.Builder
	var crawler func(*html.Node)
	crawler = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(node)
	
	result := strings.TrimSpace(text.String())
	if result == "" {
		return "未知视频"
	}
	return result
}

func (va *VideoAnalyzer) resolveURL(rawURL string, baseURL *url.URL) string {
	if rawURL == "" {
		return ""
	}
	
	// 如果是完整URL，直接返回
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	
	// 解析相对URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	
	resolved := baseURL.ResolveReference(parsedURL)
	return resolved.String()
}

func (va *VideoAnalyzer) detectVideoType(videoURL string) string {
	lower := strings.ToLower(videoURL)
	
	if strings.Contains(lower, ".m3u8") {
		return "m3u8"
	} else if strings.Contains(lower, ".mp4") {
		return "mp4"
	} else if strings.Contains(lower, ".webm") {
		return "webm"
	} else if strings.Contains(lower, ".mov") {
		return "mov"
	} else if strings.Contains(lower, ".avi") {
		return "avi"
	} else if strings.Contains(lower, ".flv") {
		return "flv"
	} else if strings.Contains(lower, ".mkv") {
		return "mkv"
	}
	
	return "unknown"
}

func (va *VideoAnalyzer) detectVideoTypeFromMime(mimeType string) string {
	mimeType = strings.ToLower(mimeType)
	
	if strings.Contains(mimeType, "mp4") {
		return "mp4"
	} else if strings.Contains(mimeType, "webm") {
		return "webm"
	} else if strings.Contains(mimeType, "m3u8") {
		return "m3u8"
	}
	
	return "unknown"
}

func (va *VideoAnalyzer) detectQuality(videoURL string) string {
	lower := strings.ToLower(videoURL)
	
	qualityPatterns := map[string]string{
		"4k":     "4K",
		"2160p":  "4K",
		"1080p":  "1080p",
		"720p":   "720p",
		"480p":   "480p",
		"360p":   "360p",
		"240p":   "240p",
		"hd":     "HD",
		"sd":     "SD",
	}
	
	for pattern, quality := range qualityPatterns {
		if strings.Contains(lower, pattern) {
			return quality
		}
	}
	
	return "unknown"
}

func (va *VideoAnalyzer) isValidVideoURL(videoURL string) bool {
	// 基本的URL验证
	if !strings.HasPrefix(videoURL, "http://") && !strings.HasPrefix(videoURL, "https://") {
		return false
	}
	
	// 检查是否为视频文件
	return va.isVideoLink(videoURL)
}

func (va *VideoAnalyzer) isVideoLink(link string) bool {
	lower := strings.ToLower(link)
	videoExtensions := []string{".mp4", ".webm", ".mov", ".avi", ".flv", ".mkv", ".m3u8"}
	
	for _, ext := range videoExtensions {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	
	return false
}

func (va *VideoAnalyzer) deduplicateVideos(videos []types.VideoResource) []types.VideoResource {
	seen := make(map[string]bool)
	var result []types.VideoResource
	
	for _, video := range videos {
		if !seen[video.URL] {
			seen[video.URL] = true
			result = append(result, video)
		}
	}
	
	return result
}