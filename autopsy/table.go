// Package autopsy finds the damage that spreadsheet programs do to CSV files
// when someone opens one and saves it again: phone numbers that lost their
// leading 0, order numbers turned into 4.11111E+15, dates rewritten or
// swapped, scrambled accents. It says what can be undone, and undoes it.
package autopsy

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// Table is a CSV file in memory.
type Table struct {
	Header   []string
	Rows     [][]string
	Comma    rune   // the delimiter: a comma, semicolon or tab
	Encoding string // "UTF-8" or "Windows-1252"
	BOM      bool   // the file started with a UTF-8 byte order mark
}

// Read parses a CSV file. It guesses the delimiter, and reads a file that is
// not valid UTF-8 as Windows-1252, which is what such files usually are.
func Read(data []byte) (*Table, error) {
	t := &Table{Encoding: "UTF-8"}
	if rest, ok := bytes.CutPrefix(data, []byte("\xef\xbb\xbf")); ok {
		data, t.BOM = rest, true
	}
	text := string(data)
	if !utf8.Valid(data) {
		text, t.Encoding = fromWindows1252(data), "Windows-1252"
	}
	t.Comma = sniff(text)
	r := csv.NewReader(strings.NewReader(text))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = t.Comma, -1, true
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("the file is empty")
	}
	t.Header, t.Rows = records[0], records[1:]
	return t, nil
}

// Write writes the table as UTF-8 CSV, with a byte order mark if the
// original had one.
func (t *Table) Write(w io.Writer) error {
	if t.BOM {
		if _, err := io.WriteString(w, "\xef\xbb\xbf"); err != nil {
			return err
		}
	}
	cw := csv.NewWriter(w)
	cw.Comma = t.Comma
	cw.Write(t.Header)
	cw.WriteAll(t.Rows)
	return cw.Error()
}

// Column returns the values in column c, with "" where a row is too short.
func (t *Table) Column(c int) []string {
	out := make([]string, len(t.Rows))
	for i, row := range t.Rows {
		if c < len(row) {
			out[i] = row[c]
		}
	}
	return out
}

// sniff picks the delimiter that appears most often in the first line,
// outside quotes.
func sniff(text string) rune {
	line, _, _ := strings.Cut(text, "\n")
	counts := map[rune]int{}
	quoted := false
	for _, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
		case !quoted && (r == ',' || r == ';' || r == '\t'):
			counts[r]++
		}
	}
	best := ','
	for _, r := range []rune{';', '\t'} {
		if counts[r] > counts[best] {
			best = r
		}
	}
	return best
}

// windows1252 maps bytes 0x80 to 0x9F of Windows-1252 to Unicode. The rest of
// the upper half is the same as Latin-1.
var windows1252 = [32]rune{
	'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8D, 'Ž', 0x8F,
	0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9D, 'ž', 'Ÿ',
}

func fromWindows1252(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		switch {
		case c < 0x80, c >= 0xA0:
			b.WriteRune(rune(c))
		default:
			b.WriteRune(windows1252[c-0x80])
		}
	}
	return b.String()
}

// toWindows1252 returns the Windows-1252 byte for r, if there is one.
func toWindows1252(r rune) (byte, bool) {
	if r < 0x100 {
		return byte(r), true // ASCII, Latin-1, and stray C1 controls
	}
	for i, w := range windows1252 {
		if w == r {
			return byte(0x80 + i), true
		}
	}
	return 0, false
}
