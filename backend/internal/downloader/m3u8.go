package downloader

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ProgressInfo struct {
	Downloaded int
	Total      int
	Current    string
}

type segment struct {
	URL      string
	Index    int
	Filename string
}

func DownloadM3U8(m3u8URL string, outputFilename string, progressCallback func(int, int)) error {
	fmt.Printf("开始下载 M3U8: %s\n", m3u8URL)

	tmpDir, err := os.MkdirTemp("", "m3u8_download_*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	segments, err := parseM3U8(m3u8URL)
	if err != nil {
		return fmt.Errorf("解析 M3U8 文件失败: %v", err)
	}

	fmt.Printf("发现 %d 个分片\n", len(segments))

	progressChan := make(chan ProgressInfo, len(segments))
	go displayProgress(progressChan, len(segments))

	err = downloadSegments(segments, tmpDir, progressChan, progressCallback)
	close(progressChan)
	if err != nil {
		return fmt.Errorf("下载分片失败: %v", err)
	}

	fmt.Println("\n开始合并分片...")
	err = mergeSegments(segments, tmpDir, outputFilename)
	if err != nil {
		return fmt.Errorf("合并分片失败: %v", err)
	}

	fmt.Printf("下载完成: %s\n", outputFilename)
	return nil
}

func parseM3U8(m3u8URL string) ([]segment, error) {
	resp, err := http.Get(m3u8URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 错误: %d", resp.StatusCode)
	}

	baseURL, err := url.Parse(m3u8URL)
	if err != nil {
		return nil, err
	}

	var segments []segment
	scanner := bufio.NewScanner(resp.Body)
	index := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		segmentURL := line
		if !strings.HasPrefix(line, "http") {
			resolvedURL, err := baseURL.Parse(line)
			if err != nil {
				return nil, fmt.Errorf("解析分片URL失败: %v", err)
			}
			segmentURL = resolvedURL.String()
		}

		filename := fmt.Sprintf("segment_%04d.ts", index)
		segments = append(segments, segment{
			URL:      segmentURL,
			Index:    index,
			Filename: filename,
		})
		index++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("M3U8 文件中未找到任何分片")
	}

	return segments, nil
}

func downloadSegments(segments []segment, tmpDir string, progressChan chan<- ProgressInfo, progressCallback func(int, int)) error {
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var downloadError error
	downloaded := 0

	for _, seg := range segments {
		wg.Add(1)
		go func(s segment) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var err error
			for retries := 0; retries < 3; retries++ {
				err = downloadSegment(s, tmpDir)
				if err == nil {
					break
				}
				if retries < 2 {
					time.Sleep(time.Duration(retries+1) * time.Second)
				}
			}
			
			mu.Lock()
			if err != nil {
				if downloadError == nil {
					downloadError = fmt.Errorf("下载分片 %s 失败: %v", s.Filename, err)
				}
			} else {
				downloaded++
				progressInfo := ProgressInfo{
					Downloaded: downloaded,
					Total:      len(segments),
					Current:    s.Filename,
				}
				progressChan <- progressInfo
				
				// 调用进度回调函数
				if progressCallback != nil {
					progressCallback(downloaded, len(segments))
				}
			}
			mu.Unlock()
		}(seg)
	}

	wg.Wait()
	return downloadError
}

func downloadSegment(seg segment, tmpDir string) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(seg.URL)
	if err != nil {
		return fmt.Errorf("下载分片 %s 失败: %v", seg.Filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("分片 %s HTTP 错误: %d", seg.Filename, resp.StatusCode)
	}

	filePath := filepath.Join(tmpDir, seg.Filename)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件 %s 失败: %v", seg.Filename, err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件 %s 失败: %v", seg.Filename, err)
	}

	return nil
}

func displayProgress(progressChan <-chan ProgressInfo, total int) {
	for progress := range progressChan {
		percentage := float64(progress.Downloaded) / float64(total) * 100
		fmt.Printf("\r下载进度: %d/%d (%.1f%%) - 当前: %s", 
			progress.Downloaded, progress.Total, percentage, progress.Current)
	}
}

func mergeSegments(segments []segment, tmpDir, outputFilename string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("未找到 ffmpeg，请先安装 ffmpeg")
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Index < segments[j].Index
	})

	listFilePath := filepath.Join(tmpDir, "filelist.txt")
	listFile, err := os.Create(listFilePath)
	if err != nil {
		return fmt.Errorf("创建文件列表失败: %v", err)
	}
	defer listFile.Close()

	for _, seg := range segments {
		segmentPath := filepath.Join(tmpDir, seg.Filename)
		if _, err := os.Stat(segmentPath); err != nil {
			return fmt.Errorf("分片文件 %s 不存在", seg.Filename)
		}
		fmt.Fprintf(listFile, "file '%s'\n", segmentPath)
	}

	cmd := exec.Command("ffmpeg", 
		"-f", "concat", 
		"-safe", "0", 
		"-i", listFilePath, 
		"-c", "copy", 
		"-y", 
		outputFilename)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}