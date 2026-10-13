// Package probe mengorkestrasi jalannya check untuk setiap target.
package probe

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"dbprobe/pkg/check"
	"dbprobe/pkg/config"
	"dbprobe/pkg/engine"
	"dbprobe/pkg/model"
)

// Version diisi oleh cmd saat build/run.
var Version = "dev"

// Options mengatur perilaku eksekusi probe.
type Options struct {
	Checks     []string      // filter check; kosong = pakai config / default
	Timeout    time.Duration // timeout per check; 0 = tanpa batas
	AllowWrite bool          // izinkan query non-SELECT
}

// Run menjalankan seluruh target dan mengembalikan Report.
func Run(ctx context.Context, cfg *config.Config, opts Options) *model.Report {
	rep := &model.Report{
		Tool:    "dbprobe",
		Version: Version,
		Time:    time.Now().Format(time.RFC3339),
	}
	for _, t := range cfg.Targets {
		rep.Results = append(rep.Results, runTarget(ctx, t, opts)...)
	}
	return rep
}

func runTarget(ctx context.Context, t config.Target, opts Options) []model.Result {
	eng, ok := engine.Get(t.Engine)
	if !ok {
		return []model.Result{{
			Target:  t.Name,
			Check:   "engine",
			Status:  model.StatusFail,
			Message: fmt.Sprintf("engine %q tidak dikenal", t.Engine),
			Hint:    "engine tersedia: " + strings.Join(engine.Names(), ", "),
		}}
	}

	// Opsional: sisipkan password dari field terpisah (di-encode otomatis).
	dsn := t.DSN
	if t.Password != "" {
		injected, ierr := injectPassword(dsn, t.Password)
		if ierr != nil {
			return []model.Result{{
				Target:  t.Name,
				Check:   "dsn",
				Status:  model.StatusFail,
				Message: "gagal menyisipkan password ke DSN",
				Err:     ierr.Error(),
				Hint:    "pastikan DSN memuat user, mis. oracle://USER@host:port/service",
			}}
		}
		dsn = injected
	}

	// DSN bergaya URL (oracle://, postgres://, sqlserver://) divalidasi lebih dulu;
	// jika gagal parse, beri hint encoding agar pesan errornya actionable.
	if strings.Contains(dsn, "://") {
		if _, uerr := url.Parse(dsn); uerr != nil {
			return []model.Result{{
				Target:  t.Name,
				Check:   "dsn",
				Status:  model.StatusFail,
				Message: "DSN tidak valid sebagai URL",
				Err:     uerr.Error(),
				Hint:    "karakter khusus pada password/DSN harus di-URL-encode (mis. # → %23), atau pakai field `password` agar dbprobe meng-encode otomatis",
			}}
		}
	}

	db, err := eng.Open(dsn)
	if err != nil {
		return []model.Result{{
			Target:  t.Name,
			Check:   "open",
			Status:  model.StatusFail,
			Message: "gagal membuka koneksi database",
			Err:     err.Error(),
		}}
	}
	defer db.Close()
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	specs, convErrs := buildQueries(t, opts)

	names := opts.Checks
	if len(names) == 0 {
		names = t.Checks
	}
	if len(names) == 0 {
		names = check.Default
	}

	c := check.Context{
		Target:  t.Name,
		Engine:  eng,
		DB:      db,
		Queries: specs,
	}

	out := make([]model.Result, 0, len(names)+len(convErrs))
	out = append(out, convErrs...)

	for _, name := range names {
		fn, ok := check.Get(name)
		if !ok {
			out = append(out, model.Result{
				Target:  t.Name,
				Check:   name,
				Status:  model.StatusSkip,
				Message: "check tidak dikenal",
				Hint:    "check tersedia: " + strings.Join(check.Names(), ", "),
			})
			continue
		}

		cctx := ctx
		var cancel context.CancelFunc
		if opts.Timeout > 0 {
			cctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		}
		rs := runSafely(cctx, name, fn, c)
		if cancel != nil {
			cancel()
		}
		for i := range rs {
			if rs[i].Target == "" {
				rs[i].Target = t.Name
			}
		}
		out = append(out, rs...)

		// Koneksi gagal => lewati check berikutnya untuk target ini.
		if name == "connectivity" && hasFail(rs) {
			out = append(out, model.Result{
				Target:  t.Name,
				Check:   "checks",
				Status:  model.StatusSkip,
				Message: "check berikutnya dilewati karena koneksi gagal",
			})
			break
		}
	}
	return out
}

// runSafely menjalankan check dengan recover() agar satu panic tidak mematikan proses.
func runSafely(ctx context.Context, name string, fn check.Func, c check.Context) (out []model.Result) {
	defer func() {
		if r := recover(); r != nil {
			out = []model.Result{{
				Target:  c.Target,
				Check:   name,
				Status:  model.StatusFail,
				Message: "panic saat menjalankan check",
				Err:     fmt.Sprint(r),
			}}
		}
	}()
	return fn(ctx, c)
}

func hasFail(rs []model.Result) bool {
	for _, r := range rs {
		if r.Status == model.StatusFail {
			return true
		}
	}
	return false
}

func buildQueries(t config.Target, opts Options) ([]check.QuerySpec, []model.Result) {
	var specs []check.QuerySpec
	var errs []model.Result
	for _, qc := range t.QueryChecks {
		spec, err := toSpec(qc, opts.AllowWrite)
		if err != nil {
			errs = append(errs, model.Result{
				Target:  t.Name,
				Check:   "query:" + qc.Name,
				Status:  model.StatusFail,
				Message: "konfigurasi query tidak valid",
				Err:     err.Error(),
			})
			continue
		}
		specs = append(specs, spec)
	}
	return specs, errs
}

func toSpec(qc config.QueryCheck, allowWrite bool) (check.QuerySpec, error) {
	spec := check.QuerySpec{
		Name:    qc.Name,
		SQL:     qc.SQL,
		Params:  qc.Params,
		MaxRows: qc.MaxRows,
	}

	readOnly := true
	switch {
	case qc.ReadOnly != nil:
		readOnly = *qc.ReadOnly
	case allowWrite:
		readOnly = false
	}
	spec.ReadOnly = readOnly

	if qc.Timeout != "" {
		d, err := time.ParseDuration(qc.Timeout)
		if err != nil {
			return spec, fmt.Errorf("timeout %q tidak valid: %w", qc.Timeout, err)
		}
		spec.Timeout = d
	}

	exp := check.Expectation{
		RowCount:   qc.Expect.RowCount,
		RowCountOp: qc.Expect.RowCountOp,
	}
	if qc.Expect.Cell != nil {
		exp.Cell = &check.CellExpect{
			Column: qc.Expect.Cell.Column,
			Row:    qc.Expect.Cell.Row,
			Op:     qc.Expect.Cell.Op,
			Value:  qc.Expect.Cell.Value,
		}
	}
	if qc.Expect.MaxDuration != "" {
		d, err := time.ParseDuration(qc.Expect.MaxDuration)
		if err != nil {
			return spec, fmt.Errorf("max_duration %q tidak valid: %w", qc.Expect.MaxDuration, err)
		}
		exp.MaxDuration = d
	}
	spec.Expect = exp
	return spec, nil
}

// injectPassword menyisipkan password ke DSN dengan encoding yang benar.
//
// Untuk DSN bergaya URL (scheme://), password di-encode otomatis via net/url
// sehingga karakter khusus seperti '#' tidak perlu ditulis manual sebagai %23.
// Untuk DSN gaya MySQL ([user[:pass]@]rest), password disisipkan apa adanya.
func injectPassword(dsn, password string) (string, error) {
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		user := ""
		if u.User != nil {
			user = u.User.Username()
		}
		u.User = url.UserPassword(user, password)
		return u.String(), nil
	}

	if at := strings.LastIndex(dsn, "@"); at != -1 {
		user := dsn[:at]
		if c := strings.Index(user, ":"); c != -1 {
			user = user[:c]
		}
		return user + ":" + password + dsn[at:], nil
	}
	return "", fmt.Errorf("DSN tidak memuat user (format [user[:password]@]host)")
}
