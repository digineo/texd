package service

import (
	"bytes"
	_ "embed"
	"net/http"
)

//go:embed ui.html
var uiHTML []byte

func HandleUI(res http.ResponseWriter, req *http.Request) {
	buf := bytes.NewBuffer(uiHTML)

	res.Header().Set("Content-Type", mimeTypeHTML)
	res.Header().Set("X-Content-Type-Options", "nosniff")
	res.WriteHeader(http.StatusOK)
	buf.WriteTo(res)
}
