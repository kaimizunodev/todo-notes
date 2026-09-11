// Package handler adalah "delivery layer" — jembatan HTTP ke usecase.
// Tugasnya cuma: baca request, panggil usecase, tulis response.
// Tidak ada logika bisnis di sini, semua sudah ada di usecase.
package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"todo-list/internal/domain"
	"todo-list/internal/notifier"
	"todo-list/internal/usecase"
)

type Handler struct {
	uc       *usecase.JudulUsecase
	notifier notifier.Notifier
	staticFS fs.FS
}

func New(uc *usecase.JudulUsecase, n notifier.Notifier, staticFS fs.FS) *Handler {
	return &Handler{uc: uc, notifier: n, staticFS: staticFS}
}

func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/judul", h.handleJudulCollection)
	mux.HandleFunc("/api/judul/", h.handleJudulSub)
	mux.HandleFunc("/api/settings", h.handleSettings)
	mux.HandleFunc("/api/export/csv", h.handleExportCSV)

	fileServer := http.FileServer(http.FS(h.staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Paksa "/" untuk selalu render static/index.html, supaya tidak
		// jatuh ke directory listing bawaan http.FileServer.
		if r.URL.Path == "/" {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
	return mux
}

// RunDeadlineChecker jalan sebagai goroutine, cek berkala & kirim notifikasi.
func (h *Handler) RunDeadlineChecker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		for _, hit := range h.uc.CekDeadline() {
			msg := hit.Tugas.Nama + " (" + hit.JudulNama + ") jatuh tempo " + hit.Tugas.Deadline
			go h.notifier.Send("Deadline tugas", msg)
		}
	}
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeUsecaseErr memetakan error dari usecase (domain.ValidationError /
// domain.NotFoundError) ke status HTTP yang sesuai.
func writeUsecaseErr(w http.ResponseWriter, err error) {
	switch err.(type) {
	case *domain.ValidationError:
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case *domain.NotFoundError:
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "Terjadi kesalahan internal.")
	}
}

// ---------- /api/judul ----------

func (h *Handler) handleJudulCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.uc.All())

	case http.MethodPost:
		var body struct {
			Nama string `json:"nama"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Body tidak valid.")
			return
		}
		j, err := h.uc.AddJudul(body.Nama)
		if err != nil {
			writeUsecaseErr(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, j)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- /api/judul/{id}[/tugas[/{tugasId}[/status]]] ----------

func (h *Handler) handleJudulSub(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/judul/")
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		writeError(w, http.StatusBadRequest, "ID Judul tidak ada.")
		return
	}
	judulID, err := strconv.Atoi(segs[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID Judul harus berupa angka.")
		return
	}

	// /api/judul/{id}
	if len(segs) == 1 {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
			return
		}
		if err := h.uc.DeleteJudul(judulID); err != nil {
			writeUsecaseErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Judul berhasil dihapus."})
		return
	}

	if segs[1] != "tugas" {
		writeError(w, http.StatusNotFound, "Route tidak ditemukan.")
		return
	}

	// /api/judul/{id}/tugas
	if len(segs) == 2 {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
			return
		}
		var in usecase.TugasInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "Body tidak valid.")
			return
		}
		j, t, err := h.uc.AddTugas(judulID, in)
		if err != nil {
			writeUsecaseErr(w, err)
			return
		}
		go h.notifier.Send("Tugas baru", t.Nama+" ditambahkan ke \""+j.Nama+"\"")
		writeJSON(w, http.StatusCreated, t)
		return
	}

	tugasID, err := strconv.Atoi(segs[2])
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID Tugas harus berupa angka.")
		return
	}

	// /api/judul/{id}/tugas/{tugasId}/status
	if len(segs) == 4 && segs[3] == "status" {
		if r.Method != http.MethodPatch {
			writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Body tidak valid.")
			return
		}
		t, err := h.uc.UpdateTugasStatus(judulID, tugasID, body.Status)
		if err != nil {
			writeUsecaseErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, t)
		return
	}

	// /api/judul/{id}/tugas/{tugasId}
	if len(segs) == 3 {
		switch r.Method {
		case http.MethodPut:
			var in usecase.TugasInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeError(w, http.StatusBadRequest, "Body tidak valid.")
				return
			}
			t, err := h.uc.EditTugas(judulID, tugasID, in)
			if err != nil {
				writeUsecaseErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, t)
			return

		case http.MethodDelete:
			if err := h.uc.DeleteTugas(judulID, tugasID); err != nil {
				writeUsecaseErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "Tugas berhasil dihapus."})
			return

		default:
			writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
			return
		}
	}

	writeError(w, http.StatusNotFound, "Route tidak ditemukan.")
}

// ---------- /api/settings ----------

func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]string{"ntfyTopic": h.notifier.Topic()})

	case http.MethodPost:
		var body struct {
			NtfyTopic string `json:"ntfyTopic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Body tidak valid.")
			return
		}
		h.notifier.SetTopic(body.NtfyTopic)
		writeJSON(w, http.StatusOK, map[string]string{"ntfyTopic": h.notifier.Topic()})

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- /api/export/csv ----------

// handleExportCSV meratakan semua Judul+Tugas jadi satu tabel CSV, satu baris per Tugas.
func (h *Handler) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
		return
	}

	filename := fmt.Sprintf("buku-tugas-%s.csv", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	// BOM supaya karakter non-ASCII tampil benar kalau dibuka di Excel Windows
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	cw := csv.NewWriter(w)
	cw.Write([]string{"Judul", "Tugas", "Rincian", "Prioritas", "PIC", "Deadline", "Status"})

	for _, j := range h.uc.All() {
		if len(j.Tugas) == 0 {
			cw.Write([]string{j.Nama, "", "", "", "", "", ""})
			continue
		}
		for _, t := range j.Tugas {
			cw.Write([]string{
				j.Nama,
				t.Nama,
				t.Rincian,
				priorityLabel(t.Priority),
				t.Pic,
				t.Deadline,
				t.Status,
			})
		}
	}
	cw.Flush()
}

func priorityLabel(p int) string {
	switch p {
	case 1:
		return "Tinggi"
	case 2:
		return "Sedang"
	case 3:
		return "Rendah"
	default:
		return strconv.Itoa(p)
	}
}
