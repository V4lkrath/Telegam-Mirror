package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ---------- Media Helpers ----------
func getMediaPath(channelUsername, postID string, mediaIndex int, ext string) string {
	safeUsername := strings.ToLower(channelUsername)
	hash := md5.Sum([]byte(fmt.Sprintf("%s_%d", postID, mediaIndex)))
	hashStr := hex.EncodeToString(hash[:])[:16]
	filename := fmt.Sprintf("%s_%s%s", postID, hashStr, ext)
	return filepath.Join("media", safeUsername, filename)
}

func downloadMedia(url, localPath string) (string, string, error) {
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("mkdir failed: %w", err)
	}
	if _, err := os.Stat(localPath); err == nil {
		file, _ := os.Open(localPath)
		defer file.Close()
		buf := make([]byte, 512)
		file.Read(buf)
		mime := http.DetectContentType(buf)
		return localPath, mime, nil
	}
	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://t.me/")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad HTTP status: %d", resp.StatusCode)
	}
	out, err := os.Create(localPath)
	if err != nil {
		return "", "", fmt.Errorf("create failed: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", "", fmt.Errorf("write failed: %w", err)
	}
	file, _ := os.Open(localPath)
	defer file.Close()
	buf := make([]byte, 512)
	file.Read(buf)
	mime := http.DetectContentType(buf)
	return localPath, mime, nil
}

// ---------- Fetch Channel Data ----------
func fetchChannelData(username string) (*ChannelData, error) {
	delay := time.Duration(2+rand.Intn(3)) * time.Second
	fmt.Printf("  - Waiting %v before request...\n", delay)
	time.Sleep(delay)

	url := fmt.Sprintf("https://t.me/s/%s", username)
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36",
	}
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,fa;q=0.8")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	html := string(body)

	channelInfo := extractChannelInfo(html, username)
	posts := extractPostsUsingGoquery(html, username)

	return &ChannelData{
		Info:        channelInfo,
		Posts:       posts,
		LastUpdated: time.Now().Unix(),
	}, nil
}

// ---------- Extract Channel Info ----------
func extractChannelInfo(html, username string) ChannelInfo {
	titleRe := regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)
	title := username
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		title = matches[1]
	}
	photoRe := regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
	photo := ""
	if matches := photoRe.FindStringSubmatch(html); len(matches) > 1 {
		photo = matches[1]
	}
	descRe := regexp.MustCompile(`<meta property="og:description" content="([^"]*)"`)
	description := ""
	if matches := descRe.FindStringSubmatch(html); len(matches) > 1 {
		description = matches[1]
	}
	return ChannelInfo{
		ID:          0,
		Title:       title,
		Username:    username,
		Photo:       photo,
		Description: description,
	}
}

// ---------- Extract Posts and Download Media ----------
func extractPostsUsingGoquery(html string, channelUsername string) []Post {
	var posts []Post
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		fmt.Printf("Error parsing HTML: %v\n", err)
		return posts
	}

	doc.Find(".tgme_widget_message_wrap").Each(func(i int, postDiv *goquery.Selection) {
		// Post ID
		postIDStr, _ := postDiv.Attr("data-post")
		var postID int64
		if postIDStr != "" {
			parts := strings.Split(postIDStr, "/")
			if len(parts) > 1 {
				postID, _ = strconv.ParseInt(parts[1], 10, 64)
			}
		}
		if postID == 0 {
			postID = int64(i + 1)
		}

		// Message text
		message := strings.TrimSpace(postDiv.Find(".tgme_widget_message_text").Text())

		// Date
		var postDate time.Time
		if dt, exists := postDiv.Find("time").Attr("datetime"); exists {
			postDate, _ = time.Parse("2006-01-02T15:04:05", dt)
		}
		if postDate.IsZero() {
			postDate = time.Now()
		}

		// Views
		views := 0
		viewStr := strings.TrimSpace(postDiv.Find(".tgme_widget_message_views").Text())
		if viewStr != "" {
			if strings.HasSuffix(viewStr, "K") {
				viewStr = strings.TrimSuffix(viewStr, "K")
				if v, err := strconv.ParseFloat(viewStr, 64); err == nil {
					views = int(v * 1000)
				}
			} else {
				views, _ = strconv.Atoi(viewStr)
			}
		}

		var mediaList []Media

		// Photos
		postDiv.Find("img.tgme_widget_message_photo").Each(func(idx int, img *goquery.Selection) {
			src, exists := img.Attr("src")
			if !exists || src == "" {
				return
			}
			ext := ".jpg"
			if strings.Contains(src, ".png") {
				ext = ".png"
			}
			localPath := getMediaPath(channelUsername, fmt.Sprintf("%d", postID), idx, ext)
			savedPath, mime, err := downloadMedia(src, localPath)
			if err != nil {
				fmt.Printf("  Failed to download photo: %v\n", err)
				return
			}
			fmt.Printf("  Downloaded photo: %s -> %s\n", src, savedPath)
			mediaList = append(mediaList, Media{
				Type:      "photo",
				URL:       src,
				LocalPath: savedPath,
				MimeType:  mime,
			})
		})

		// Videos
		postDiv.Find("a.tgme_widget_message_video_player").Each(func(idx int, a *goquery.Selection) {
			href, exists := a.Attr("href")
			if !exists || href == "" {
				return
			}
			localPath := getMediaPath(channelUsername, fmt.Sprintf("%d", postID), idx+100, ".mp4")
			savedPath, mime, err := downloadMedia(href, localPath)
			if err != nil {
				fmt.Printf("  Failed to download video: %v\n", err)
				return
			}
			fmt.Printf("  Downloaded video: %s -> %s\n", href, savedPath)
			mediaList = append(mediaList, Media{
				Type:      "video",
				URL:       href,
				LocalPath: savedPath,
				MimeType:  mime,
			})
		})

		post := Post{
			ID:       postID,
			Message:  message,
			Date:     postDate,
			Views:    views,
			Media:    mediaList,
			Hashtags: extractHashtags(message),
			Mentions: extractMentions(message),
			Links:    extractLinks(message),
		}
		posts = append(posts, post)
	})
	return posts
}

// ---------- Helpers ----------
func extractHashtags(text string) []string {
	re := regexp.MustCompile(`#\w+`)
	return re.FindAllString(text, -1)
}
func extractMentions(text string) []string {
	re := regexp.MustCompile(`@\w+`)
	return re.FindAllString(text, -1)
}
func extractLinks(text string) []string {
	re := regexp.MustCompile(`https?://[^\s]+`)
	return re.FindAllString(text, -1)
}
