package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	fileMap   = make(map[string]string)
	fileMapMu sync.RWMutex
	uploadDir = filepath.Join(".", "uploads")
)

func randomID() string {
	b := make([]byte, 6)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "6003"
	}

	err := os.MkdirAll(uploadDir, 0755)
	if err != nil {
		panic(err)
	}

	r := gin.Default()

	r.POST("/upload", handleUpload)
	r.GET("/files/:id", handleGetFile)

	fmt.Printf("CDN Server running on port %s\n", port)
	r.Run(":" + port)
}

func handleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "no file uploaded",
		})
		return
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file.Filename), "."))

	var id string
	for {
		if ext != "" {
			id = fmt.Sprintf("%s.%s", randomID(), ext)
		} else {
			id = randomID()
		}

		fileMapMu.RLock()
		_, exists := fileMap[id]
		fileMapMu.RUnlock()

		if !exists {
			break
		}
	}

	filePath := filepath.Join(uploadDir, id)

	if err := c.SaveUploadedFile(file, filePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to save file",
		})
		return
	}

	fileMapMu.Lock()
	fileMap[id] = filePath
	fileMapMu.Unlock()

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	c.JSON(http.StatusOK, gin.H{
		"url":      fmt.Sprintf("%s://%s/files/%s", scheme, c.Request.Host, id),
		"fileSize": file.Size,
	})
}

func handleGetFile(c *gin.Context) {
	id := c.Param("id")

	fileMapMu.RLock()
	filePath, exists := fileMap[id]
	fileMapMu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "file not found",
		})
		return
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "file not found",
		})
		return
	}

	f, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to open file",
		})
		return
	}
	defer f.Close()

	ext := filepath.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		buff := make([]byte, 512)
		n, _ := f.Read(buff)
		contentType = http.DetectContentType(buff[:n])
		_, _ = f.Seek(0, 0)
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	c.Header("Cache-Control", "public, max-age=31536000")

	_, err = io.Copy(c.Writer, f)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
}
