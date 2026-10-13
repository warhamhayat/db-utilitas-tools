// Package config memuat konfigurasi target dari file YAML.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config adalah root konfigurasi.
type Config struct {
	Targets []Target `yaml:"targets"`
}

// Target adalah satu database yang akan diuji.
type Target struct {
	Name   string `yaml:"name"`
	Engine string `yaml:"engine"`
	DSN    string `yaml:"dsn"`
	// Password opsional. Bila diisi, akan disisipkan ke DSN dan di-URL-encode
	// otomatis (sehingga karakter khusus seperti '#' tidak perlu di-encode manual).
	Password string   `yaml:"password"`
	Checks   []string `yaml:"checks"`

	QueryChecks []QueryCheck `yaml:"query_checks"`
}

// QueryCheck adalah definisi satu query custom + ekspektasi (opsional).
type QueryCheck struct {
	Name     string         `yaml:"name"`
	SQL      string         `yaml:"sql"`
	File     string         `yaml:"file"`
	Params   map[string]any `yaml:"params"`
	Timeout  string         `yaml:"timeout"`
	MaxRows  int            `yaml:"max_rows"`
	ReadOnly *bool          `yaml:"readonly"`
	Expect   Expectation    `yaml:"expect"`
}

// Expectation adalah aturan validasi hasil query (semua opsional).
type Expectation struct {
	RowCount    *int        `yaml:"row_count"`
	RowCountOp  string      `yaml:"row_count_op"`
	Cell        *CellExpect `yaml:"cell"`
	MaxDuration string      `yaml:"max_duration"`
}

// CellExpect memvalidasi nilai satu sel.
type CellExpect struct {
	Column string `yaml:"column"`
	Row    int    `yaml:"row"`
	Op     string `yaml:"op"`
	Value  any    `yaml:"value"`
}

// Load membaca file konfigurasi, mengekspansi ${ENV} pada DSN/password, dan
// memuat isi file SQL eksternal (relatif terhadap lokasi config).
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baca config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	baseDir := filepath.Dir(path)
	for i := range cfg.Targets {
		t := &cfg.Targets[i]
		t.DSN = os.ExpandEnv(t.DSN)
		t.Password = os.ExpandEnv(t.Password)
		for j := range t.QueryChecks {
			qc := &t.QueryChecks[j]
			if qc.File != "" && qc.SQL == "" {
				content, err := os.ReadFile(filepath.Join(baseDir, qc.File))
				if err != nil {
					return nil, fmt.Errorf("target %q, query %q: baca file SQL: %w", t.Name, qc.Name, err)
				}
				qc.SQL = string(content)
			}
		}
	}
	return &cfg, nil
}

// Validate memastikan setiap target punya field wajib.
func (c *Config) Validate() error {
	if len(c.Targets) == 0 {
		return fmt.Errorf("tidak ada target di config")
	}
	for i, t := range c.Targets {
		if t.Name == "" {
			return fmt.Errorf("target #%d tidak punya name", i+1)
		}
		if t.Engine == "" {
			return fmt.Errorf("target %q tidak punya engine", t.Name)
		}
		if t.DSN == "" {
			return fmt.Errorf("target %q tidak punya dsn", t.Name)
		}
	}
	return nil
}
