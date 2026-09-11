// Package usecase berisi seluruh logika bisnis (aturan validasi, alur
// tambah/edit/hapus). Bergantung pada domain dan pada interface
// repository.JudulRepository — tidak tahu soal HTTP atau format JSON di kabel.
package usecase

import (
	"strings"
	"sync"
	"time"

	"todo-list/internal/domain"
	"todo-list/internal/repository"
)

// TugasInput adalah data yang dibutuhkan untuk membuat/mengedit Tugas.
type TugasInput struct {
	Nama     string `json:"nama"`
	Rincian  string `json:"rincian"`
	Priority int    `json:"priority"`
	Pic      string `json:"pic"`
	Deadline string `json:"deadline"`
}

func (in *TugasInput) validate() error {
	if strings.TrimSpace(in.Nama) == "" {
		return domain.NewValidationError("Nama Tugas tidak boleh kosong.")
	}
	if in.Priority < 1 || in.Priority > 3 {
		return domain.NewValidationError("Prioritas harus 1, 2, atau 3.")
	}
	return nil
}

// JudulUsecase mengurus seluruh aturan bisnis terhadap Judul & Tugas.
// Data di-cache di memori (in.data) dan disinkronkan ke repository tiap berubah.
type JudulUsecase struct {
	mu   sync.Mutex
	repo repository.JudulRepository
	data []domain.Judul
}

func NewJudulUsecase(repo repository.JudulRepository) *JudulUsecase {
	u := &JudulUsecase{repo: repo}
	if data, err := repo.Load(); err == nil {
		u.data = data
	}
	return u
}

func (u *JudulUsecase) persist() {
	_ = u.repo.Save(u.data) // kesalahan I/O di sini bersifat non-fatal untuk request
}

func (u *JudulUsecase) All() []domain.Judul {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.data
}

func (u *JudulUsecase) findJudulIndex(id int) int {
	for i := range u.data {
		if u.data[i].ID == id {
			return i
		}
	}
	return -1
}

func (u *JudulUsecase) AddJudul(nama string) (domain.Judul, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	nama = strings.TrimSpace(nama)
	if nama == "" {
		return domain.Judul{}, domain.NewValidationError("Nama Judul tidak boleh kosong.")
	}
	newID := 1
	if len(u.data) > 0 {
		newID = u.data[len(u.data)-1].ID + 1
	}
	j := domain.Judul{ID: newID, Nama: nama, Tugas: []domain.Tugas{}}
	u.data = append(u.data, j)
	u.persist()
	return j, nil
}

func (u *JudulUsecase) DeleteJudul(id int) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	idx := u.findJudulIndex(id)
	if idx == -1 {
		return domain.NewNotFoundError("ID Judul tidak ditemukan.")
	}
	u.data = append(u.data[:idx], u.data[idx+1:]...)
	u.persist()
	return nil
}

// AddTugas menambahkan Tugas ke Judul tertentu. Return Judul (untuk nama) & Tugas baru.
func (u *JudulUsecase) AddTugas(judulID int, in TugasInput) (domain.Judul, domain.Tugas, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	idx := u.findJudulIndex(judulID)
	if idx == -1 {
		return domain.Judul{}, domain.Tugas{}, domain.NewNotFoundError("ID Judul tidak ditemukan.")
	}
	if err := in.validate(); err != nil {
		return domain.Judul{}, domain.Tugas{}, err
	}
	j := &u.data[idx]
	t := domain.Tugas{
		ID:       j.NextTugasID(),
		Nama:     strings.TrimSpace(in.Nama),
		Rincian:  strings.TrimSpace(in.Rincian),
		Priority: in.Priority,
		Pic:      strings.TrimSpace(in.Pic),
		Deadline: strings.TrimSpace(in.Deadline),
		Status:   "Belum Selesai",
	}
	j.Tugas = append(j.Tugas, t)
	u.persist()
	return *j, t, nil
}

func (u *JudulUsecase) EditTugas(judulID, tugasID int, in TugasInput) (domain.Tugas, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	idx := u.findJudulIndex(judulID)
	if idx == -1 {
		return domain.Tugas{}, domain.NewNotFoundError("ID Judul tidak ditemukan.")
	}
	j := &u.data[idx]
	tIdx := j.FindTugasIndex(tugasID)
	if tIdx == -1 {
		return domain.Tugas{}, domain.NewNotFoundError("ID Tugas tidak ditemukan.")
	}
	if err := in.validate(); err != nil {
		return domain.Tugas{}, err
	}
	t := &j.Tugas[tIdx]
	t.Nama = strings.TrimSpace(in.Nama)
	t.Rincian = strings.TrimSpace(in.Rincian)
	t.Priority = in.Priority
	t.Pic = strings.TrimSpace(in.Pic)
	newDeadline := strings.TrimSpace(in.Deadline)
	if newDeadline != t.Deadline {
		t.Notified = false
	}
	t.Deadline = newDeadline
	u.persist()
	return *t, nil
}

func (u *JudulUsecase) UpdateTugasStatus(judulID, tugasID int, status string) (domain.Tugas, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if status != "Selesai" && status != "Belum Selesai" {
		return domain.Tugas{}, domain.NewValidationError("Status harus 'Selesai' atau 'Belum Selesai'.")
	}
	idx := u.findJudulIndex(judulID)
	if idx == -1 {
		return domain.Tugas{}, domain.NewNotFoundError("ID Judul tidak ditemukan.")
	}
	j := &u.data[idx]
	tIdx := j.FindTugasIndex(tugasID)
	if tIdx == -1 {
		return domain.Tugas{}, domain.NewNotFoundError("ID Tugas tidak ditemukan.")
	}
	j.Tugas[tIdx].Status = status
	u.persist()
	return j.Tugas[tIdx], nil
}

func (u *JudulUsecase) DeleteTugas(judulID, tugasID int) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	idx := u.findJudulIndex(judulID)
	if idx == -1 {
		return domain.NewNotFoundError("ID Judul tidak ditemukan.")
	}
	j := &u.data[idx]
	tIdx := j.FindTugasIndex(tugasID)
	if tIdx == -1 {
		return domain.NewNotFoundError("ID Tugas tidak ditemukan.")
	}
	j.Tugas = append(j.Tugas[:tIdx], j.Tugas[tIdx+1:]...)
	u.persist()
	return nil
}

// CekDeadline memindai tugas yang jatuh tempo & belum dinotifikasi,
// menandainya sebagai sudah dinotifikasi, lalu mengembalikan daftarnya.
func (u *JudulUsecase) CekDeadline() []domain.DeadlineHit {
	u.mu.Lock()
	defer u.mu.Unlock()

	var hits []domain.DeadlineHit
	today := time.Now()
	changed := false
	for i := range u.data {
		for k := range u.data[i].Tugas {
			t := &u.data[i].Tugas[k]
			if t.IsSelesai() || t.Notified || !t.IsJatuhTempo(today) {
				continue
			}
			hits = append(hits, domain.DeadlineHit{JudulNama: u.data[i].Nama, Tugas: *t})
			t.Notified = true
			changed = true
		}
	}
	if changed {
		u.persist()
	}
	return hits
}
