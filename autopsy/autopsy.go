package autopsy

import (
	"fmt"
	"strings"
)

// Fixable says whether damage can be undone.
type Fixable string

const (
	Repairable Fixable = "can be repaired"
	Lost       Fixable = "cannot be undone"
)

// Finding is one kind of damage in one column, or in the whole file.
type Finding struct {
	Kind     string    `json:"kind"`
	Column   string    `json:"column,omitempty"` // empty when it is about the whole file
	Title    string    `json:"title"`
	Detail   string    `json:"detail"`
	Advice   string    `json:"advice"`
	Fixable  Fixable   `json:"fixable"`
	Rows     []int     `json:"rows,omitempty"` // row numbers as a spreadsheet shows them: the header is row 1
	Examples []Example `json:"examples,omitempty"`

	col   int            // the column, or -1 for the whole file
	fixes map[int]string // repaired values, by data row
}

// Example is one damaged cell, and what it should be when that is known.
type Example struct {
	Row   int    `json:"row"`
	Found string `json:"found"`
	Fixed string `json:"fixed,omitempty"`
}

// detectors look at one column at a time.
var detectors = []func(header string, vals []string) *Finding{
	scrambled, leadingZeros, scientific, cutDigits, serialDates, mixedDates, textDates, cellLimit,
}

// Examine looks for damage in every column, and in the file as a whole.
func Examine(t *Table) []Finding {
	var out []Finding
	if t.Encoding != "UTF-8" {
		out = append(out, Finding{
			Kind: "encoding", col: -1, Fixable: Repairable,
			Title:  "The file is saved as " + t.Encoding + ", not UTF-8",
			Detail: "Accented letters such as ñ and é break when a file like this is opened as UTF-8, which most programs now expect.",
			Advice: "Repair saves the file as UTF-8.",
		})
	}
	out = append(out, limits(t)...)
	for c, header := range t.Header {
		vals := t.Column(c)
		for _, detect := range detectors {
			if f := detect(header, vals); f != nil {
				f.col, f.Column = c, header
				out = append(out, *f)
			}
		}
	}
	return out
}

// Repair returns a copy of t with every repairable finding undone, and how
// many cells it changed. The copy is UTF-8 whatever the original was.
func Repair(t *Table, findings []Finding) (*Table, int) {
	out := &Table{Header: t.Header, Comma: t.Comma, Encoding: "UTF-8", BOM: t.BOM}
	out.Rows = make([][]string, len(t.Rows))
	for i, row := range t.Rows {
		out.Rows[i] = append([]string(nil), row...)
	}
	changed := 0
	for _, f := range findings {
		if f.Fixable != Repairable || f.col < 0 {
			continue
		}
		for i, v := range f.fixes {
			if f.col < len(out.Rows[i]) && out.Rows[i][f.col] != v {
				out.Rows[i][f.col] = v
				changed++
			}
		}
	}
	return out, changed
}

// limits spots a file cut off at a spreadsheet's size limits.
func limits(t *Table) []Finding {
	var out []Finding
	if rows := len(t.Rows) + 1; rows == 1_048_576 || rows == 65_536 {
		out = append(out, Finding{
			Kind: "row-limit", col: -1, Fixable: Lost,
			Title:  fmt.Sprintf("The file stops at exactly %s rows, a spreadsheet's limit", thousands(rows)),
			Detail: fmt.Sprintf("Excel cannot hold more than %s rows, so anything after that was probably cut off when the file was saved.", thousands(rows)),
			Advice: "Export the data again in smaller parts, or open it with a tool that has no row limit.",
		})
	}
	if cols := len(t.Header); cols == 16_384 || cols == 256 {
		out = append(out, Finding{
			Kind: "column-limit", col: -1, Fixable: Lost,
			Title:  fmt.Sprintf("The file has exactly %s columns, a spreadsheet's limit", thousands(cols)),
			Detail: "Columns after that were probably cut off when the file was saved.",
			Advice: "Export the data again, or open it with a tool that has no column limit.",
		})
	}
	return out
}

// damage collects the damaged cells of one column as a detector finds them.
type damage struct {
	rows  []int
	ex    []Example
	fixes map[int]string
}

func (d *damage) add(i int, found, fixed string) {
	d.rows = append(d.rows, i+2)
	if len(d.ex) < 3 {
		d.ex = append(d.ex, Example{Row: i + 2, Found: found, Fixed: fixed})
	}
	if fixed != "" {
		if d.fixes == nil {
			d.fixes = map[int]string{}
		}
		d.fixes[i] = fixed
	}
}

func (d *damage) finding(kind string, fixable Fixable, title, detail, advice string) *Finding {
	return &Finding{Kind: kind, Fixable: fixable, Title: title, Detail: detail, Advice: advice,
		Rows: d.rows, Examples: d.ex, fixes: d.fixes}
}

// noun guesses what a column holds from its name, for plain-English titles.
func noun(header string) string {
	h := strings.ToLower(header)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(h, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("phone", "mobile", "tel", "contact", "cell", "whatsapp", "viber", "msisdn"):
		return "phone numbers"
	case has("date", "time", "_at", "_on", "dob", "birth", "day"):
		return "dates"
	case has("zip"):
		return "ZIP codes"
	case has("postal", "postcode"):
		return "postal codes"
	case has("card"):
		return "card numbers"
	case has("tracking", "waybill", "awb", "shipment"):
		return "tracking numbers"
	case has("order"):
		return "order numbers"
	case has("account", "acct"):
		return "account numbers"
	case has("name"):
		return "names"
	case has("address", "street", "barangay", "city", "province"):
		return "addresses"
	case h == "id" || strings.HasSuffix(h, "_id") || strings.HasSuffix(h, " id") || strings.HasSuffix(h, "id"):
		return "IDs"
	}
	return "values"
}

var singular = map[string]string{
	"phone numbers": "phone number", "dates": "date", "ZIP codes": "ZIP code", "postal codes": "postal code",
	"card numbers": "card number", "tracking numbers": "tracking number", "order numbers": "order number",
	"account numbers": "account number", "names": "name", "addresses": "address", "IDs": "ID",
	"values": "value", "numbers": "number", "cells": "cell", "problems": "problem",
}

// count writes "1 phone number" or "312 phone numbers".
func count(n int, plural string) string {
	if n == 1 {
		return "1 " + singular[plural]
	}
	return thousands(n) + " " + plural
}

func their(n int) string { return pick(n, "its", "their") }
func be(n int) string    { return pick(n, "was", "were") }

func pick(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// thousands writes 1048576 as 1,048,576.
func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
