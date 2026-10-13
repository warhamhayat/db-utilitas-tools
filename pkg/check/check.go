// Package check berisi implementasi setiap pemeriksaan (check).
//
// dbprobe hanya menyediakan dua check:
//
//	connectivity — memastikan bisa connect ke database (ping).
//	query        — menjalankan SQL yang ditulis pengguna sendiri.
//
// Dengan begitu tidak ada SQL yang di-hardcode: semua kebutuhan lain
// (list tabel, jumlah koneksi, daftar user, dsb.) ditulis sebagai query_checks.
package check

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"dbprobe/pkg/engine"
	"dbprobe/pkg/model"
)

// Context membawa semua yang dibutuhkan sebuah check.
type Context struct {
	Target  string
	Engine  engine.Engine
	DB      *sql.DB
	Queries []QuerySpec
}

// Func menjalankan sebuah check dan mengembalikan hasil (bisa lebih dari satu).
type Func func(ctx context.Context, c Context) []model.Result

var registry = map[string]Func{}

// Register mendaftarkan sebuah check. Dipanggil dari init().
func Register(name string, f Func) { registry[name] = f }

// Get mengambil check berdasarkan nama.
func Get(name string) (Func, bool) {
	f, ok := registry[name]
	return f, ok
}

// Names mengembalikan daftar check yang terdaftar (terurut).
func Names() []string {
	out := make([]string, 0, len(registry))
	for name := range registry {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Default adalah set check default bila config tidak menyebutkan apa pun.
var Default = []string{"connectivity", "query"}

func init() {
	Register("connectivity", connectivity)
}

// connectivity menguji apakah bisa connect ke database + mengukur latency.
// Tidak ada query yang dijalankan (hanya ping), sesuai prinsip sederhana.
func connectivity(ctx context.Context, c Context) []model.Result {
	start := time.Now()
	res := model.Result{Target: c.Target, Check: "connectivity"}

	if err := c.DB.PingContext(ctx); err != nil {
		res.Status = model.StatusFail
		res.Message = "gagal connect ke database"
		res.Err = err.Error()
		res.SetDuration(time.Since(start))
		return []model.Result{res}
	}

	res.Status = model.StatusOK
	res.Message = "koneksi OK"
	res.SetDuration(time.Since(start))
	res.Data = map[string]any{"latency_ms": res.DurationMS}
	return []model.Result{res}
}
