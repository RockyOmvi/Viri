package explorer

import (
	"embed"
	"net/http"
)

//go:embed index.html
var explorerHTML embed.FS

func Handler() http.Handler {
	return http.FileServer(http.FS(explorerHTML))
}
