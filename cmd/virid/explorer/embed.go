package explorer

import (
	"embed"
	"net/http"
	"strings"
)

//go:embed index.html
var explorerHTML embed.FS

func Handler() http.Handler {
	fileServer := http.FileServer(http.FS(explorerHTML))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ".") {
			fileServer.ServeHTTP(w, r)
			return
		}
		data, err := explorerHTML.ReadFile("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
}
