package service

import (
	"bytes"
	"embed"
	"io"
	"net/http"
)

//go:embed ui.html
var uiHTML []byte

//go:embed assets/*
var assets embed.FS

func HandleUI(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", mimeTypeHTML)
	res.Header().Set("X-Content-Type-Options", "nosniff")
	res.WriteHeader(http.StatusOK)

	_, _ = io.Copy(res, bytes.NewReader(uiHTML))
}

func HandleAssets() http.Handler {
	return http.FileServer(http.FS(assets))
}
