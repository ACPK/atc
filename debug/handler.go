package debug

import (
	"net/http"

	"github.com/tedsuo/rata"
)

type VersionsDB interface {
	GetVersionInfo() string
}

func NewHandler() (http.Handler, error) {
	handlers := map[string]http.Handler{
		GetVersionsDB: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
		}),
	}
	return rata.NewRouter(Routes, handlers)
}
