// Package report memformat hasil probe ke tabel (terminal) atau JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"dbprobe/pkg/model"
)

// Write menulis report ke w dengan format "table" atau "json".
func Write(w io.Writer, rep *model.Report, format string, color bool) error {
	switch format {
	case "json":
		return writeJSON(w, rep)
	case "", "table":
		return writeTable(w, rep, color)
	default:
		return fmt.Errorf("format tidak dikenal: %q (pakai table|json)", format)
	}
}

func writeJSON(w io.Writer, rep *model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func writeTable(w io.Writer, rep *model.Report, color bool) error {
	fmt.Fprintf(w, "%s %s  ·  %s\n\n", rep.Tool, rep.Version, rep.Time)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TARGET\tCHECK\tSTATUS\tMESSAGE")
	for _, r := range rep.Results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			r.Target, r.Check, statusLabel(r.Status, color), messageText(r))
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	s := rep.Summary()
	fmt.Fprintf(w, "\nSummary: %d OK · %d WARN · %d SKIP · %d FAIL\n",
		s[model.StatusOK], s[model.StatusWarn], s[model.StatusSkip], s[model.StatusFail])

	// Tampilkan hint unik (mis. grant yang dibutuhkan).
	seen := map[string]bool{}
	for _, r := range rep.Results {
		if r.Hint != "" && !seen[r.Hint] {
			seen[r.Hint] = true
			fmt.Fprintf(w, "  hint: %s\n", r.Hint)
		}
	}

	// Render hasil yang bersifat tabular (mis. rows dari check query).
	for _, r := range rep.Results {
		t, ok := r.Data.(model.Tabular)
		if !ok {
			continue
		}
		rows := t.TableRows()
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s  %s  (%d baris)\n", r.Target, r.Check, len(rows))
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, strings.Join(t.TableColumns(), "\t"))
		for _, row := range rows {
			cells := make([]string, len(row))
			for i, c := range row {
				cells[i] = model.ToString(c)
			}
			fmt.Fprintln(tw, strings.Join(cells, "\t"))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func messageText(r model.Result) string {
	msg := r.Message
	if r.Err != "" {
		if msg != "" {
			msg += ": "
		}
		msg += r.Err
	}
	return msg
}

func statusLabel(s model.Status, color bool) string {
	sym := map[model.Status]string{
		model.StatusOK:   "✅",
		model.StatusWarn: "⚠️ ",
		model.StatusSkip: "⏭️ ",
		model.StatusFail: "❌",
	}[s]

	label := string(s)
	if !color {
		return label
	}
	code := map[model.Status]string{
		model.StatusOK:   "\x1b[32m", // green
		model.StatusWarn: "\x1b[33m", // yellow
		model.StatusSkip: "\x1b[36m", // cyan
		model.StatusFail: "\x1b[31m", // red
	}[s]
	const reset = "\x1b[0m"
	return sym + " " + code + label + reset
}
