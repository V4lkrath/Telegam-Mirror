package main

import "time"

type ChannelInfo struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Username string `json:"username"`
	Photo    string `json:"photo_url"`
	Description string `json:"description"`
}

type Media struct {
	Type      string `json:"type"` // photo, video, document, audio
	URL       string `json:"url"`
	Caption   string `json:"caption,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Duration  int    `json:"duration,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	FileSize  int64  `json:"file_size,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
}

type Post struct {
	ID          int64     `json:"id"`
	Message     string    `json:"message"`
	Caption     string    `json:"caption,omitempty"`
	Date        time.Time `json:"date"`
	Edited      bool      `json:"edited"`
	EditDate    time.Time `json:"edit_date,omitempty"`
	Views       int       `json:"views"`
	Forwards    int       `json:"forwards"`
	Replies     int       `json:"replies"`
	SenderID    int64     `json:"sender_id"`
	SenderName  string    `json:"sender_name"`
	Media       []Media   `json:"media,omitempty"`
	Hashtags    []string  `json:"hashtags,omitempty"`
	Mentions    []string  `json:"mentions,omitempty"`
	Links       []string  `json:"links,omitempty"`
}

type ChannelData struct {
	Info         ChannelInfo `json:"info"`
	Posts        []Post      `json:"posts"`
	LastUpdated  int64       `json:"last_updated"`
}
