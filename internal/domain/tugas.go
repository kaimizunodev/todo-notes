package domain

import "time"

// Tugas merepresentasikan satu tugas di dalam sebuah Judul
type Tugas struct {
	ID       int    `json:"id"`
	Nama     string `json:"nama"`
	Rincian  string `json:"rincian"`
	Priority int    `json:"priority"`
	Pic      string `json:"pic"`
	Deadline string `json:"deadline"`
	Status   string `json:"status"` // "Belum Selesai" atau "Selesai"
	Notified bool   `json:"notified"`
}

func (t *Tugas) IsSelesai() bool { return t.Status == "Selesai" }

// IsJatuhTempo mengecek apakah deadline tugas ini sudah hari ini atau lewat.
func (t *Tugas) IsJatuhTempo(today time.Time) bool {
	if t.Deadline == "" {
		return false
	}
	dl, err := time.Parse("2006-01-02", t.Deadline)
	if err != nil {
		return false
	}
	return !dl.After(today)
}

// Judul merepresentasikan satu rangkaian/kategori tugas, berisi banyak Tugas
type Judul struct {
	ID    int     `json:"id"`
	Nama  string  `json:"nama"`
	Tugas []Tugas `json:"tugas"`
}

func (j *Judul) FindTugasIndex(tugasID int) int {
	for i := range j.Tugas {
		if j.Tugas[i].ID == tugasID {
			return i
		}
	}
	return -1
}

func (j *Judul) NextTugasID() int {
	if len(j.Tugas) == 0 {
		return 1
	}
	return j.Tugas[len(j.Tugas)-1].ID + 1
}

// DeadlineHit merepresentasikan satu tugas yang jatuh tempo dan perlu dinotifikasi.
type DeadlineHit struct {
	JudulNama string
	Tugas     Tugas
}
