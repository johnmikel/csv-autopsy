package autopsy

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Checks lists, in plain words, everything Examine looks for.
const Checks = "lost leading zeros, numbers in scientific notation, cut-off digits, dates turned into numbers, " +
	"rewritten or swapped dates, text turned into dates, scrambled letters, the wrong text encoding, " +
	"and files cut off at a spreadsheet's limits"

// Report is everything Examine found in one file.
type Report struct {
	File       string    `json:"file"`
	Rows       int       `json:"rows"`
	Columns    int       `json:"columns"`
	Repairable int       `json:"repairable"`
	Findings   []Finding `json:"findings"`
}

// NewReport sums up the findings for one file.
func NewReport(file string, t *Table, findings []Finding) Report {
	r := Report{File: file, Rows: len(t.Rows), Columns: len(t.Header), Findings: findings}
	for _, f := range findings {
		if f.Fixable == Repairable {
			r.Repairable++
		}
	}
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	return r
}

// JSON writes the report as JSON.
func (r Report) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// Text writes the report in plain English, in colour if color is set.
func (r Report) Text(w io.Writer, color bool) {
	paint := func(code, s string) string {
		if !color {
			return s
		}
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	fmt.Fprintf(w, "%s: %s rows, %s columns\n\n", paint("1", r.File), thousands(r.Rows), thousands(r.Columns))
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "No damage found.")
		fmt.Fprintln(w, wrap("Checked for "+Checks+".", "", 78))
		return
	}
	fmt.Fprintf(w, "Found %s. %s\n", count(len(r.Findings), "problems"), repairLine(r))
	for _, f := range r.Findings {
		where := f.Column
		if where == "" {
			where = "Whole file"
		}
		code := "32" // green: can be repaired
		if f.Fixable == Lost {
			code = "31" // red: cannot be undone
		}
		fmt.Fprintf(w, "\n%s %s\n", paint("1", where+": "+f.Title), paint(code, "["+string(f.Fixable)+"]"))
		fmt.Fprintln(w, wrap(f.Detail, "    ", 78))
		if len(f.Rows) > 0 {
			fmt.Fprintln(w, wrap(pick(len(f.Rows), "Row ", "Rows ")+rowList(f.Rows)+".", "    ", 78))
		}
		fmt.Fprintln(w, wrap(f.Advice, "    ", 78))
	}
	fmt.Fprintln(w)
}

func repairLine(r Report) string {
	switch {
	case r.Repairable == 0:
		return "None of it can be repaired automatically."
	case r.Repairable == len(r.Findings):
		return "All of it can be repaired with -fix."
	}
	return fmt.Sprintf("%d of them can be repaired with -fix.", r.Repairable)
}

// rowList writes the first few row numbers and how many more there are.
func rowList(rows []int) string {
	var parts []string
	for _, r := range rows[:min(5, len(rows))] {
		parts = append(parts, fmt.Sprint(r))
	}
	list := strings.Join(parts, ", ")
	if more := len(rows) - len(parts); more > 0 {
		return list + " and " + thousands(more) + " more"
	}
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
	return list
}

// wrap breaks text into lines of at most width characters, each starting with indent.
func wrap(text, indent string, width int) string {
	var lines []string
	line := indent
	for _, word := range strings.Fields(text) {
		if len([]rune(line))+len([]rune(word)) > width && strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimRight(line, " "))
			line = indent
		}
		line += word + " "
	}
	return strings.Join(append(lines, strings.TrimRight(line, " ")), "\n")
}
