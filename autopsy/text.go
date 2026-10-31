package autopsy

import (
	"fmt"
	"unicode/utf8"
)

// scrambled finds text with scrambled letters, as when Peñaflor comes back
// as PeÃ±aflor: UTF-8 text read back as Windows-1252, sometimes twice over.
func scrambled(header string, vals []string) *Finding {
	var d damage
	for i, v := range vals {
		if fixed, ok := unscramble(v); ok {
			d.add(i, v, fixed)
		}
	}
	if len(d.rows) == 0 {
		return nil
	}
	n, ex := len(d.rows), d.ex[0]
	return d.finding("scrambled", Repairable,
		fmt.Sprintf("%s %s scrambled letters", count(n, noun(header)), pick(n, "has", "have")),
		fmt.Sprintf("\"%s\" should read \"%s\". The text was saved as UTF-8 and read back as Windows-1252 somewhere along the way.", ex.Found, ex.Fixed),
		"Repair unscrambles them.")
}

// unscramble undoes UTF-8 read as Windows-1252, up to three times over.
func unscramble(s string) (string, bool) {
	out := s
	for range 3 {
		next, ok := unscrambleOnce(out)
		if !ok {
			break
		}
		out = next
	}
	return out, out != s
}

// unscrambleOnce turns each character back into the Windows-1252 byte it was
// read from and reads the bytes as UTF-8. Text that was never scrambled
// does not survive the trip: José becomes the bytes "Jos\xe9", which are not
// UTF-8, so it is left alone.
func unscrambleOnce(s string) (string, bool) {
	suspect := false
	b := make([]byte, 0, len(s))
	for _, r := range s {
		if r >= 0xC2 && r <= 0xF4 {
			suspect = true // how the first byte of a UTF-8 letter looks in Windows-1252: Â to ô
		}
		c, ok := toWindows1252(r)
		if !ok {
			return s, false
		}
		b = append(b, c)
	}
	if !suspect || !utf8.Valid(b) || string(b) == s {
		return s, false
	}
	return string(b), true
}

// cellLimit finds text cut off at 32,767 characters, the most a spreadsheet cell holds.
func cellLimit(header string, vals []string) *Finding {
	var d damage
	for i, v := range vals {
		if utf8.RuneCountInString(v) == 32_767 {
			d.add(i, string([]rune(v)[:40])+"…", "")
		}
	}
	if len(d.rows) == 0 {
		return nil
	}
	n := len(d.rows)
	return d.finding("cell-limit", Lost,
		fmt.Sprintf("%s stop at exactly 32,767 characters", count(n, "cells")),
		"That is the most text a spreadsheet cell can hold, so the rest was cut off.",
		"Get the full text again from the system that made the file.")
}
