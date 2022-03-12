package service

import (
	"bytes"
	"embed"
	"net/http"
)

//go:embed ui.html
var uiHTML []byte

//go:embed assets/*
var assets embed.FS

func HandleUI(res http.ResponseWriter, req *http.Request) {
	buf := bytes.NewBuffer(uiHTML)

	res.Header().Set("Content-Type", mimeTypeHTML)
	res.Header().Set("X-Content-Type-Options", "nosniff")
	res.WriteHeader(http.StatusOK)
	buf.WriteTo(res)
}

func HandleAssets() http.Handler {
	return http.FileServer(http.FS(assets))
}
