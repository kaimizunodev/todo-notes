package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"todo-list/internal/handler"
	"todo-list/internal/notifier"
	"todo-list/internal/repository"
	"todo-list/internal/usecase"
)

//go:embed static
var staticFiles embed.FS

func main() {
	// --- wiring: repository -> usecase -> handler (dependency mengarah ke dalam) ---
	judulRepo := repository.NewJSONJudulRepository("todos.json")
	judulUC := usecase.NewJudulUsecase(judulRepo)
	ntfy := notifier.NewNtfyNotifier("config.json")

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal("Gagal load folder static (embed):", err)
	}

	h := handler.New(judulUC, ntfy, staticSub)
	go h.RunDeadlineChecker()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Println("Server jalan di http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, h.Routes()))
}
