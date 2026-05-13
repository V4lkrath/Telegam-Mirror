package main

import (
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
)

func getMediaPathForChannel(username string, fileID string) string {
    safeUsername := strings.ToLower(username)
    return filepath.Join("media", safeUsername, fileID)
}

func downloadAndSaveMedia(url string, savePath string) (string, error) {
    dir := filepath.Dir(savePath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return "", fmt.Errorf("failed to create media directory: %v", err)
    }

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Get(url)
    if err != nil {
        return "", fmt.Errorf("download failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("bad HTTP status: %d", resp.StatusCode)
    }

    out, err := os.Create(savePath)
    if err != nil {
        return "", fmt.Errorf("failed to create file: %v", err)
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to save file: %v", err)
    }

    return savePath, nil
}
