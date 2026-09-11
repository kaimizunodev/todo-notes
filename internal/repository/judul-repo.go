// Package repository mengurus persistensi data — detail teknis "disimpan di
// mana dan bagaimana" (di sini: file JSON). Layer usecase hanya bergantung
// pada interface JudulRepository, tidak tahu implementasinya file JSON.
package repository

import (
	"encoding/json"
	"os"

	"todo-list/internal/domain"
)

// JudulRepository adalah kontrak penyimpanan data Judul/Tugas.
// Implementasi lain (mis. database) bisa dibuat tanpa mengubah usecase.
type JudulRepository interface {
	Load() ([]domain.Judul, error)
	Save(data []domain.Judul) error
}

// JSONJudulRepository menyimpan data ke file JSON lokal.
type JSONJudulRepository struct {
	path string
}

func NewJSONJudulRepository(path string) *JSONJudulRepository {
	return &JSONJudulRepository{path: path}
}

func (r *JSONJudulRepository) Load() ([]domain.Judul, error) {
	raw, err := os.ReadFile(r.path)
	if err != nil {
		// File belum ada = wajar untuk pertama kali jalan, bukan error.
		return []domain.Judul{}, nil
	}
	var data []domain.Judul
	if err := json.Unmarshal(raw, &data); err != nil {
		return []domain.Judul{}, err
	}
	return data, nil
}

func (r *JSONJudulRepository) Save(data []domain.Judul) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, raw, 0644)
}
