package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TelegramPost struct {
	ID        int64  `json:"id"`
	Message   string `json:"message,omitempty"`
	Caption   string `json:"caption,omitempty"`
	Date      int64  `json:"date"`
	Edited    int64  `json:"edited,omitempty"`
	Views     int    `json:"views,omitempty"`
	Forwards  int    `json:"forwards,omitempty"`
	Replies   struct {
		Replies int `json:"replies"`
	} `json:"replies,omitempty"`
	Sender struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"sender,omitempty"`
	Media []struct {
		Type      string `json:"type"`
		URL       string `json:"url"`
		Width     int    `json:"width,omitempty"`
		Height    int    `json:"height,omitempty"`
		Duration  int    `json:"duration,omitempty"`
		FileName  string `json:"file_name,omitempty"`
		FileSize  int64  `json:"file_size,omitempty"`
		MimeType  string `json:"mime_type,omitempty"`
	} `json:"media,omitempty"`
}

type TelegramChannel struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Username string `json:"username"`
	Photo    string `json:"photo_url"`
	Posts    []TelegramPost `json:"posts"`
}

// getMediaPath generates a unique local path for a media file based on channel, post ID, and index.
func getMediaPath(channelUsername, postID string, mediaIndex int, ext string) string {
	// Create a safe directory name
	safeUsername := strings.ToLower(channelUsername)
	// Use MD5 of postID + index to avoid collisions and special chars
	hash := md5.Sum([]byte(fmt.Sprintf("%s_%d", postID, mediaIndex)))
	hashStr := hex.EncodeToString(hash[:])[:16]
	filename := fmt.Sprintf("%s_%s%s", postID, hashStr, ext)
	return filepath.Join("media", safeUsername, filename)
}

// downloadMedia downloads a file from url and saves it to localPath.
// Returns the final local path and MIME type.
func downloadMedia(url, localPath string) (string, string, error) {
	// Create directory if not exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create media dir: %w", err)
	}

	// Check if file already exists (resume / skip)
	if _, err := os.Stat(localPath); err == nil {
		// File exists, detect MIME type from existing file
		file, _ := os.Open(localPath)
		defer file.Close()
		buff := make([]byte, 512)
		_, _ = file.Read(buff)
		mime := http.DetectContentType(buff)
		return localPath, mime, nil
	}

	// Download the file
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bad HTTP status: %d", resp.StatusCode)
	}

	// Save to disk
	out, err := os.Create(localPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to write file: %w", err)
	}

	// Detect MIME type from downloaded content
	file, _ := os.Open(localPath)
	defer file.Close()
	buff := make([]byte, 512)
	_, _ = file.Read(buff)
	mime := http.DetectContentType(buff)

	return localPath, mime, nil
}

// ---------- Modified fetchChannelData (passes username to extraction) ----------
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

	// Random User-Agent
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36",
	}
	randomUserAgent := userAgents[rand.Intn(len(userAgents))]
	req.Header.Set("User-Agent", randomUserAgent)
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

	// Extract channel info (title, photo, description)
	channelInfo := extractChannelInfo(html, username)

	// Extract posts with media downloading (pass username)
	posts := extractPostsFromHTML2(html, username)

	return &ChannelData{
		Info:        channelInfo,
		Posts:       posts,
		LastUpdated: time.Now().Unix(),
	}, nil
}

// ---------- Improved extractPostsFromHTML2 (downloads media) ----------
func extractPostsFromHTML2(html string, channelUsername string) []Post {
	// Regex to find each post wrapper
	postBlockRegex := regexp.MustCompile(`<div class="tgme_widget_message_wrap[^>]*>(.*?)</div></div></div>`)
	matches := postBlockRegex.FindAllStringSubmatch(html, -1)

	var posts []Post

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		postHTML := match[1]

		// Extract post ID from data-post attribute (inside the wrapper)
		idRe := regexp.MustCompile(`data-post="[^"]*/(\d+)"`)
		postID := int64(0)
		if idMatch := idRe.FindStringSubmatch(postHTML); len(idMatch) > 1 {
			postID, _ = strconv.ParseInt(idMatch[1], 10, 64)
		}

		// Extract message text
		messageRe := regexp.MustCompile(`<div class="tgme_widget_message_text[^>]*>(.*?)</div>`)
		message := ""
		if msgMatch := messageRe.FindStringSubmatch(postHTML); len(msgMatch) > 1 {
			// Remove HTML tags inside
			msgText := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(msgMatch[1], "")
			message = strings.TrimSpace(msgText)
		}

		// Extract date
		dateRe := regexp.MustCompile(`<time datetime="([^"]+)"`)
		var postDate time.Time
		if dateMatch := dateRe.FindStringSubmatch(postHTML); len(dateMatch) > 1 {
			postDate, _ = time.Parse("2006-01-02T15:04:05", dateMatch[1])
		}
		if postDate.IsZero() {
			postDate = time.Now()
		}

		// Extract views
		viewsRe := regexp.MustCompile(`<span class="tgme_widget_message_views">([^<]+)</span>`)
		views := 0
		if viewMatch := viewsRe.FindStringSubmatch(postHTML); len(viewMatch) > 1 {
			viewStr := strings.TrimSpace(viewMatch[1])
			if strings.HasSuffix(viewStr, "K") {
				viewStr = strings.TrimSuffix(viewStr, "K")
				if v, err := strconv.ParseFloat(viewStr, 64); err == nil {
					views = int(v * 1000)
				}
			} else {
				views, _ = strconv.Atoi(viewStr)
			}
		}

		// ----- Extract Media (Photos & Videos) -----
		var mediaList []Media

		// 1. Photos: look for img tags inside the post
		imgRe := regexp.MustCompile(`<img class="tgme_widget_message_photo[^"]*" src="([^"]+)"`)
		imgMatches := imgRe.FindAllStringSubmatch(postHTML, -1)
		for idx, imgMatch := range imgMatches {
			if len(imgMatch) < 2 {
				continue
			}
			imgURL := imgMatch[1]
			// Generate local path
			postIDStr := strconv.FormatInt(postID, 10)
			localPath := getMediaPath(channelUsername, postIDStr, idx, ".jpg")
			savedPath, mime, err := downloadMedia(imgURL, localPath)
			if err != nil {
				fmt.Printf("  Failed to download photo %s: %v\n", imgURL, err)
				continue
			}
			mediaList = append(mediaList, Media{
				Type:      "photo",
				URL:       imgURL,
				LocalPath: savedPath,
				MimeType:  mime,
			})
		}

		// 2. Videos: look for video tags or direct links
		videoRe := regexp.MustCompile(`<video[^>]+src="([^"]+)"`)
		videoMatches := videoRe.FindAllStringSubmatch(postHTML, -1)
		for idx, vidMatch := range videoMatches {
			if len(vidMatch) < 2 {
				continue
			}
			vidURL := vidMatch[1]
			postIDStr := strconv.FormatInt(postID, 10)
			localPath := getMediaPath(channelUsername, postIDStr, idx+100, ".mp4") // offset to avoid collision with photos
			savedPath, mime, err := downloadMedia(vidURL, localPath)
			if err != nil {
				fmt.Printf("  Failed to download video %s: %v\n", vidURL, err)
				continue
			}
			mediaList = append(mediaList, Media{
				Type:      "video",
				URL:       vidURL,
				LocalPath: savedPath,
				MimeType:  mime,
			})
		}

		// Build the Post object
		post := Post{
			ID:         postID,
			Message:    message,
			Date:       postDate,
			Views:      views,
			Media:      mediaList,
			Hashtags:   extractHashtags(message),
			Mentions:   extractMentions(message),
			Links:      extractLinks(message),
		}
		posts = append(posts, post)
	}

	return posts
}

// ---------- Helper functions remain the same ----------
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
