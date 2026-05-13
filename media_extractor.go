package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// downloadMedia downloads a file from url to localPath, returns localPath and mime
func downloadMedia(url, localPath string) (string, string, error) {
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	// if already exists, just return
	if _, err := os.Stat(localPath); err == nil {
		// detect mime
		f, _ := os.Open(localPath)
		defer f.Close()
		buf := make([]byte, 512)
		f.Read(buf)
		mime := http.DetectContentType(buf)
		return localPath, mime, nil
	}
	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("http %d", resp.StatusCode)
	}
	out, err := os.Create(localPath)
	if err != nil {
		return "", "", err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", "", err
	}
	// detect mime
	f, _ := os.Open(localPath)
	defer f.Close()
	buf := make([]byte, 512)
	f.Read(buf)
	mime := http.DetectContentType(buf)
	return localPath, mime, nil
}

// extractAndDownloadMedia uses goquery to find media inside a post's HTML
func extractAndDownloadMedia(postHTML string, postID int64, channelUsername string) []Media {
	var mediaList []Media
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(postHTML))
	if err != nil {
		return mediaList
	}
	// Photos
	doc.Find("img.tgme_widget_message_photo").Each(func(idx int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}
		ext := ".jpg"
		if strings.Contains(src, ".png") {
			ext = ".png"
		}
		path := filepath.Join("media", channelUsername, fmt.Sprintf("%d_%d%s", postID, idx, ext))
		savedPath, mime, err := downloadMedia(src, path)
		if err != nil {
			fmt.Printf("  Failed to download photo: %v\n", err)
			return
		}
		mediaList = append(mediaList, Media{
			Type:      "photo",
			URL:       src,
			LocalPath: savedPath,
			MimeType:  mime,
		})
	})
	// Videos
	doc.Find("a.tgme_widget_message_video_player").Each(func(idx int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		path := filepath.Join("media", channelUsername, fmt.Sprintf("%d_video_%d.mp4", postID, idx))
		savedPath, mime, err := downloadMedia(href, path)
		if err != nil {
			fmt.Printf("  Failed to download video: %v\n", err)
			return
		}
		mediaList = append(mediaList, Media{
			Type:      "video",
			URL:       href,
			LocalPath: savedPath,
			MimeType:  mime,
		})
	})
	return mediaList
}
