package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

//go:embed static
var staticFiles embed.FS

// Tugas merepresentasikan satu tugas di dalam sebuah Judul
type Tugas struct {
	ID       int    `json:"id"`
	Nama     string `json:"nama"`
	Rincian  string `json:"rincian"`
	Priority int    `json:"priority"`
	Pic      string `json:"pic"`
	Deadline string `json:"deadline"`
	Status   string `json:"status"` // "Belum Selesai" atau "Selesai"
}

// Judul merepresentasikan satu rangkaian/kategori tugas, berisi banyak Tugas
type Judul struct {
	ID    int     `json:"id"`
	Nama  string  `json:"nama"`
	Tugas []Tugas `json:"tugas"`
}

const dataFile = "todos.json"

var (
	mu   sync.Mutex
	data []Judul
)

func main() {
	data = loadData()

	staticSub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal("Gagal load folder static (embed):", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/judul", handleJudulCollection)
	mux.HandleFunc("/api/judul/", handleJudulSub)
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	addr := ":8080"
	log.Println("Server jalan di http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ---------- persistence ----------

func loadData() []Judul {
	var d []Judul
	raw, err := os.ReadFile(dataFile)
	if err != nil {
		return d
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		log.Println("Gagal membaca data, mulai dari list kosong:", err)
		return []Judul{}
	}
	return d
}

func saveData() {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Println("Gagal encode data:", err)
		return
	}
	if err := os.WriteFile(dataFile, raw, 0644); err != nil {
		log.Println("Gagal menulis file:", err)
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

func findJudulIndex(id int) int {
	for i := range data {
		if data[i].ID == id {
			return i
		}
	}
	return -1
}

func findTugasIndex(judulIdx int, tugasID int) int {
	for i := range data[judulIdx].Tugas {
		if data[judulIdx].Tugas[i].ID == tugasID {
			return i
		}
	}
	return -1
}

// ---------- /api/judul ----------

func handleJudulCollection(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, data)

	case http.MethodPost:
		var body struct {
			Nama string `json:"nama"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Body tidak valid.")
			return
		}
		body.Nama = strings.TrimSpace(body.Nama)
		if body.Nama == "" {
			writeError(w, http.StatusUnprocessableEntity, "Nama Judul tidak boleh kosong.")
			return
		}
		newID := 1
		if len(data) > 0 {
			newID = data[len(data)-1].ID + 1
		}
		j := Judul{ID: newID, Nama: body.Nama, Tugas: []Tugas{}}
		data = append(data, j)
		saveData()
		writeJSON(w, http.StatusCreated, j)

	default:
		writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
	}
}

// ---------- /api/judul/{id}[/tugas[/{tugasId}[/status]]] ----------

func handleJudulSub(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

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
	idx := findJudulIndex(judulID)
	if idx == -1 {
		writeError(w, http.StatusNotFound, "ID Judul tidak ditemukan.")
		return
	}

	// /api/judul/{id}
	if len(segs) == 1 {
		if r.Method == http.MethodDelete {
			data = append(data[:idx], data[idx+1:]...)
			saveData()
			writeJSON(w, http.StatusOK, map[string]string{"message": "Judul berhasil dihapus."})
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
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
		var body struct {
			Nama     string `json:"nama"`
			Rincian  string `json:"rincian"`
			Priority int    `json:"priority"`
			Pic      string `json:"pic"`
			Deadline string `json:"deadline"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "Body tidak valid.")
			return
		}
		body.Nama = strings.TrimSpace(body.Nama)
		if body.Nama == "" {
			writeError(w, http.StatusUnprocessableEntity, "Nama Tugas tidak boleh kosong.")
			return
		}
		if body.Priority < 1 || body.Priority > 3 {
			writeError(w, http.StatusUnprocessableEntity, "Prioritas harus 1, 2, atau 3.")
			return
		}
		list := data[idx].Tugas
		newID := 1
		if len(list) > 0 {
			newID = list[len(list)-1].ID + 1
		}
		t := Tugas{
			ID:       newID,
			Nama:     body.Nama,
			Rincian:  strings.TrimSpace(body.Rincian),
			Priority: body.Priority,
			Pic:      strings.TrimSpace(body.Pic),
			Deadline: strings.TrimSpace(body.Deadline),
			Status:   "Belum Selesai",
		}
		data[idx].Tugas = append(list, t)
		saveData()
		writeJSON(w, http.StatusCreated, t)
		return
	}

	// /api/judul/{id}/tugas/{tugasId}[/status]
	tugasID, err := strconv.Atoi(segs[2])
	if err != nil {
		writeError(w, http.StatusBadRequest, "ID Tugas harus berupa angka.")
		return
	}
	tIdx := findTugasIndex(idx, tugasID)
	if tIdx == -1 {
		writeError(w, http.StatusNotFound, "ID Tugas tidak ditemukan.")
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
		if body.Status != "Selesai" && body.Status != "Belum Selesai" {
			writeError(w, http.StatusUnprocessableEntity, "Status harus 'Selesai' atau 'Belum Selesai'.")
			return
		}
		data[idx].Tugas[tIdx].Status = body.Status
		saveData()
		writeJSON(w, http.StatusOK, data[idx].Tugas[tIdx])
		return
	}

	// /api/judul/{id}/tugas/{tugasId}
	if len(segs) == 3 {
		switch r.Method {
		case http.MethodPut:
			var body struct {
				Nama     string `json:"nama"`
				Rincian  string `json:"rincian"`
				Priority int    `json:"priority"`
				Pic      string `json:"pic"`
				Deadline string `json:"deadline"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "Body tidak valid.")
				return
			}
			body.Nama = strings.TrimSpace(body.Nama)
			if body.Nama == "" {
				writeError(w, http.StatusUnprocessableEntity, "Nama Tugas tidak boleh kosong.")
				return
			}
			if body.Priority < 1 || body.Priority > 3 {
				writeError(w, http.StatusUnprocessableEntity, "Prioritas harus 1, 2, atau 3.")
				return
			}
			t := &data[idx].Tugas[tIdx]
			t.Nama = body.Nama
			t.Rincian = strings.TrimSpace(body.Rincian)
			t.Priority = body.Priority
			t.Pic = strings.TrimSpace(body.Pic)
			t.Deadline = strings.TrimSpace(body.Deadline)
			saveData()
			writeJSON(w, http.StatusOK, *t)
			return

		case http.MethodDelete:
			data[idx].Tugas = append(data[idx].Tugas[:tIdx], data[idx].Tugas[tIdx+1:]...)
			saveData()
			writeJSON(w, http.StatusOK, map[string]string{"message": "Tugas berhasil dihapus."})
			return

		default:
			writeError(w, http.StatusMethodNotAllowed, "Method tidak didukung.")
			return
		}
	}

	writeError(w, http.StatusNotFound, "Route tidak ditemukan.")
}
