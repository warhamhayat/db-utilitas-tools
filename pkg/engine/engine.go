// Package engine mendefinisikan abstraksi multi-engine dan registry-nya.
//
// dbprobe sengaja dibuat sederhana: sebuah engine hanya perlu tahu cara
// membuka koneksi (Open) dan format placeholder parameter (Placeholder).
// Semua pemeriksaan lain dilakukan lewat query yang ditulis pengguna
// sendiri (lihat pkg/check), sehingga tidak ada SQL yang di-hardcode.
package engine

import (
	"database/sql"
	"sort"
)

// Engine adalah antarmuka minimal yang harus dipenuhi setiap driver DB.
type Engine interface {
	Name() string
	Open(dsn string) (*sql.DB, error)
	// Placeholder mengembalikan bind placeholder untuk posisi 1-based.
	Placeholder(pos int) string
}

var registry = map[string]func() Engine{}

// Register mendaftarkan factory sebuah engine. Dipanggil dari init().
func Register(name string, factory func() Engine) { registry[name] = factory }

// Get mengambil engine berdasarkan nama.
func Get(name string) (Engine, bool) {
	factory, ok := registry[name]
	if !ok {
		return nil, false
	}
	return factory(), true
}

// Names mengembalikan daftar engine yang terdaftar (terurut).
func Names() []string {
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
