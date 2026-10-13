// Package model berisi tipe data bersama (result, status, report) untuk dbprobe.
package model

import (
	"fmt"
	"time"
)

// Status adalah hasil akhir dari satu check.
type Status string

const (
	StatusOK   Status = "OK"
	StatusWarn Status = "WARN"
	StatusSkip Status = "SKIP" // tidak bisa dijalankan (mis. permission / capability)
	StatusFail Status = "FAIL"
)

// Result adalah hasil menjalankan satu check pada satu target.
type Result struct {
	Target     string        `json:"target"`
	Check      string        `json:"check"`
	Status     Status        `json:"status"`
	Message    string        `json:"message,omitempty"`
	Hint       string        `json:"hint,omitempty"`
	Duration   time.Duration `json:"-"`
	DurationMS int64         `json:"duration_ms"`
	Data       any           `json:"data,omitempty"`
	Err        string        `json:"error,omitempty"`
}

// SetDuration mengisi Duration dan DurationMS sekaligus.
func (r *Result) SetDuration(d time.Duration) {
	r.Duration = d
	r.DurationMS = d.Milliseconds()
}

// Report adalah kumpulan hasil dari semua target.
type Report struct {
	Tool    string   `json:"tool"`
	Version string   `json:"version"`
	Time    string   `json:"time"`
	Results []Result `json:"results"`
}

// Count menghitung jumlah result dengan status tertentu.
func (r *Report) Count(s Status) int {
	n := 0
	for _, res := range r.Results {
		if res.Status == s {
			n++
		}
	}
	return n
}

// HasFailures true bila ada minimal satu check yang FAIL.
func (r *Report) HasFailures() bool { return r.Count(StatusFail) > 0 }

// Summary mengembalikan ringkasan jumlah per status.
func (r *Report) Summary() map[Status]int {
	return map[Status]int{
		StatusOK:   r.Count(StatusOK),
		StatusWarn: r.Count(StatusWarn),
		StatusSkip: r.Count(StatusSkip),
		StatusFail: r.Count(StatusFail),
	}
}

// Tabular diimplementasikan oleh tipe Data hasil check yang dapat dirender
// sebagai tabel (mis. hasil query custom).
type Tabular interface {
	TableColumns() []string
	TableRows() [][]any
}

// ToString mengubah nilai sel apa pun menjadi string untuk ditampilkan.
func ToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
