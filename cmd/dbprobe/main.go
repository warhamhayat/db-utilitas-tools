// Command dbprobe adalah CLI untuk testing & inspeksi database multi-engine.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"dbprobe/pkg/check"
	"dbprobe/pkg/config"
	"dbprobe/pkg/engine"
	"dbprobe/pkg/model"
	"dbprobe/pkg/probe"
	"dbprobe/pkg/report"
)

var version = "0.1.0"

func main() {
	probe.Version = version
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "dbprobe",
		Short:         "dbprobe — utilitas testing & inspeksi database multi-engine",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newCheckCmd(), newPingCmd(), newQueryCmd(), newEnginesCmd())
	return root
}

// ---- check ----

func newCheckCmd() *cobra.Command {
	var (
		configPath string
		format     string
		outPath    string
		noColor    bool
		target     string
		checks     []string
		ci         bool
		allowWrite bool
		timeout    time.Duration
	)

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Jalankan check pada target",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return err
			}
			if target != "" {
				filtered, err := filterTargets(cfg, target)
				if err != nil {
					return err
				}
				cfg.Targets = filtered
			}

			rep := probe.Run(cmd.Context(), cfg, probe.Options{
				Checks:     checks,
				Timeout:    timeout,
				AllowWrite: allowWrite,
			})

			w, closeFn, err := outputWriter(outPath)
			if err != nil {
				return err
			}
			defer closeFn()
			if err := report.Write(w, rep, format, colorEnabled(noColor, outPath)); err != nil {
				return err
			}

			if ci && rep.HasFailures() {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "configs/example.yaml", "path file konfigurasi")
	cmd.Flags().StringVar(&format, "format", "table", "format output: table|json")
	cmd.Flags().StringVar(&outPath, "out", "", "tulis output ke file (default stdout)")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "matikan warna pada output tabel")
	cmd.Flags().StringVar(&target, "target", "", "hanya jalankan untuk target tertentu")
	cmd.Flags().StringSliceVar(&checks, "checks", nil, "daftar check (mis. connectivity,users)")
	cmd.Flags().BoolVar(&ci, "ci", false, "exit code 1 bila ada check FAIL")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false, "izinkan query non-SELECT pada check query")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "timeout per check (mis. 15s; 0 = tanpa batas)")
	return cmd
}

// ---- ping ----

func newPingCmd() *cobra.Command {
	var (
		configPath string
		target     string
		format     string
		noColor    bool
	)

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Uji konektivitas saja (mirip ping DB)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if target != "" {
				filtered, err := filterTargets(cfg, target)
				if err != nil {
					return err
				}
				cfg.Targets = filtered
			}
			rep := probe.Run(cmd.Context(), cfg, probe.Options{Checks: []string{"connectivity"}})
			return report.Write(os.Stdout, rep, format, colorEnabled(noColor, ""))
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "configs/example.yaml", "path file konfigurasi")
	cmd.Flags().StringVar(&target, "target", "", "hanya jalankan untuk target tertentu")
	cmd.Flags().StringVar(&format, "format", "table", "format output: table|json")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "matikan warna")
	return cmd
}

// ---- query (ad-hoc) ----

func newQueryCmd() *cobra.Command {
	var (
		configPath string
		target     string
		sqlText    string
		filePath   string
		format     string
		noColor    bool
		allowWrite bool
		maxRows    int
	)

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Jalankan query custom ke satu target (ad-hoc)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if sqlText == "" && filePath == "" {
				return errors.New("perlu --sql atau --file")
			}
			if sqlText == "" {
				raw, err := os.ReadFile(filePath)
				if err != nil {
					return err
				}
				sqlText = string(raw)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if target == "" {
				return errors.New("perlu --target")
			}
			tgt, err := findTarget(cfg, target)
			if err != nil {
				return err
			}

			eng, ok := engine.Get(tgt.Engine)
			if !ok {
				return fmt.Errorf("engine %q tidak dikenal (tersedia: %s)", tgt.Engine, strings.Join(engine.Names(), ", "))
			}
			db, err := eng.Open(tgt.DSN)
			if err != nil {
				return err
			}
			defer db.Close()

			spec := check.QuerySpec{
				Name:     "adhoc",
				SQL:      sqlText,
				ReadOnly: !allowWrite,
				Timeout:  30 * time.Second,
				MaxRows:  maxRows,
			}
			res := check.RunQuery(cmd.Context(), tgt.Name, eng, db, spec)
			rep := &model.Report{
				Tool:    "dbprobe",
				Version: version,
				Time:    time.Now().Format(time.RFC3339),
				Results: []model.Result{res},
			}
			if err := report.Write(os.Stdout, rep, format, colorEnabled(noColor, "")); err != nil {
				return err
			}
			if res.Status == model.StatusFail {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "configs/example.yaml", "path file konfigurasi")
	cmd.Flags().StringVar(&target, "target", "", "nama target di config")
	cmd.Flags().StringVar(&sqlText, "sql", "", "query SQL yang dijalankan")
	cmd.Flags().StringVar(&filePath, "file", "", "baca query dari file .sql")
	cmd.Flags().StringVar(&format, "format", "json", "format output: table|json")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "matikan warna")
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false, "izinkan query non-SELECT")
	cmd.Flags().IntVar(&maxRows, "max-rows", 0, "batas baris hasil (0 = default 1000)")
	return cmd
}

// ---- engines ----

func newEnginesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "engines",
		Short: "Tampilkan engine dan check yang tersedia",
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Printf("Engine : %s\n", strings.Join(engine.Names(), ", "))
			fmt.Printf("Checks : %s\n", strings.Join(check.Names(), ", "))
			fmt.Printf("Default: %s\n", strings.Join(check.Default, ", "))
			return nil
		},
	}
}

// ---- helpers ----

func filterTargets(cfg *config.Config, name string) ([]config.Target, error) {
	for _, t := range cfg.Targets {
		if t.Name == name {
			return []config.Target{t}, nil
		}
	}
	return nil, fmt.Errorf("target %q tidak ditemukan di config", name)
}

func findTarget(cfg *config.Config, name string) (config.Target, error) {
	for _, t := range cfg.Targets {
		if t.Name == name {
			return t, nil
		}
	}
	return config.Target{}, fmt.Errorf("target %q tidak ditemukan di config", name)
}

func outputWriter(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func colorEnabled(noColor bool, outPath string) bool {
	if noColor || outPath != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
