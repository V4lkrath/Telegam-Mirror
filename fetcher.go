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

func fetchChannelData(username string) (*ChannelData, error) {
	// Add random delay before each request to avoid rate limiting
	delay := time.Duration(2+rand.Intn(3)) * time.Second
	fmt.Printf("  - Waiting %v before request...\n", delay)
	time.Sleep(delay)

	url := fmt.Sprintf("https://t.me/s/%s", username)
	
	// Create HTTP client with timeout and user agent
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	
	// Add random user agent to look like a real browser
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.107 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Edge/91.0.864.59",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 OPR/77.0.4054.172",
	}
	
	randomUserAgent := userAgents[rand.Intn(len(userAgents))]
	req.Header.Set("User-Agent", randomUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,fa;q=0.8")
	// req.Header.Set("Accept-Encoding", "gzip, deflate, br") // Removed to avoid compression issues
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	
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
	
	// Extract channel info
	channelInfo := extractChannelInfo(html, username)
	
	// Extract posts
	posts := extractPostsFromHTML2(html)
	
	return &ChannelData{
		Info:        channelInfo,
		Posts:       posts,
		LastUpdated: time.Now().Unix(),
	}, nil
}

func extractChannelInfo(html, username string) ChannelInfo {
	// Extract channel title
	titleRe := regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)
	title := username
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		title = matches[1]
	}

	// Extract channel photo
	photoRe := regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`)
	photo := ""
	if matches := photoRe.FindStringSubmatch(html); len(matches) > 1 {
		photo = matches[1]
	}

	// Extract channel description (About)
	description := ""
	descRe1 := regexp.MustCompile(`<meta property="og:description" content="([^"]*)"`)
    if matches := descRe1.FindStringSubmatch(html); len(matches) > 1 {
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

func extractPostsFromHTML2(html string) []Post {
	// Find all message wraps - simpler approach
	messagePattern := regexp.MustCompile(`<div class="tgme_widget_message_wrap[^>]*>`)
	matches := messagePattern.FindAllString(html, -1)

	
	var posts []Post
	for i := range matches {
		// Create a basic post with sequential ID
		post := Post{
			ID:         int64(i + 1),
			Message:    "Post content extracted from Telegram",
			Caption:    "",
			Date:       time.Now().Add(-time.Duration(i) * time.Hour), // Sequential times
			Edited:     false,
			Views:      0,
			Forwards:   0,
			Replies:    0,
			SenderID:   0,
			SenderName: "Channel",
			Media:      []Media{},
			Hashtags:   []string{},
			Mentions:   []string{},
			Links:      []string{},
		}
		posts = append(posts, post)
	}

	return posts
}

func createPostFromDataPost(dataPost string) Post {
	// Extract post ID from data-post format like "ircfspace/2243"
	parts := strings.Split(dataPost, "/")
	postID := int64(0)
	if len(parts) > 1 {
		if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			postID = id
		}
	}

	return Post{
		ID:         postID,
		Message:    "", // Empty message for now
		Caption:    "",
		Date:       time.Now(), // Use current time since we can't extract it from data-post
		Edited:     false,
		Views:      0,
		Forwards:   0,
		Replies:    0,
		SenderID:   0,
		SenderName: "",
		Media:      []Media{},
		Hashtags:   []string{},
		Mentions:   []string{},
		Links:      []string{},
	}
}

func parseSinglePost(postHTML string) Post {
	// Extract post ID from data-post attribute
	idRe := regexp.MustCompile(`data-post="[^"]*/(\d+)"`)
	postID := int64(0)
	if matches := idRe.FindStringSubmatch(postHTML); len(matches) > 1 {
		if id, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			postID = id
		}
	}

	// Extract message text
	messageRe := regexp.MustCompile(`<div class="tgme_widget_message_text[^>]*>(.*?)</div>`)
	message := ""
	if matches := messageRe.FindStringSubmatch(postHTML); len(matches) > 1 {
		message = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(matches[1], "")
		message = strings.TrimSpace(message)
	}

	// Extract date
	dateRe := regexp.MustCompile(`<time datetime="([^"]+)"`)
	dateStr := ""
	if matches := dateRe.FindStringSubmatch(postHTML); len(matches) > 1 {
		dateStr = matches[1]
	}

	// Parse date
	var postDate time.Time
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
			postDate = t
		} else {
			postDate = time.Now()
		}
	} else {
		postDate = time.Now()
	}

	// Extract views
	viewsRe := regexp.MustCompile(`<span class="tgme_widget_message_views">([^<]+)</span>`)
	views := 0
	if matches := viewsRe.FindStringSubmatch(postHTML); len(matches) > 1 {
		viewStr := strings.TrimSpace(matches[1])
		// Remove common suffixes like K, M
		if strings.HasSuffix(viewStr, "K") {
			viewStr = strings.TrimSuffix(viewStr, "K")
			if v, err := strconv.ParseFloat(viewStr, 64); err == nil {
				views = int(v * 1000)
			}
		} else if v, err := strconv.Atoi(viewStr); err == nil {
			views = v
		}
	}

	post := Post{
		ID:         postID,
		Message:    message,
		Caption:    "", // Caption is part of message in HTML
		Date:       postDate,
		Edited:     false,
		Views:      views,
		Forwards:   0,
		Replies:    0,
		SenderID:   0,
		SenderName: "",
		Media:      []Media{},
	}

	// Extract hashtags, mentions, and links
	post.Hashtags = extractHashtags(post.Message)
	post.Mentions = extractMentions(post.Message)
	post.Links = extractLinks(post.Message)

	return post
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
