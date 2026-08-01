package handler

import (
	"log/slog"
	"net/http"
	"os"

	"settletally/internal/service"
)

var app = service.NewHandler(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

func Handler(writer http.ResponseWriter, request *http.Request) {
	app.ServeHTTP(writer, request)
}
