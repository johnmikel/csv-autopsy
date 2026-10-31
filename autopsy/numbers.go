package autopsy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// leadingZeros finds numbers that lost the 0 they started with, as phone
// numbers and ZIP codes do when a spreadsheet takes them for numbers.
//
// The evidence is either values in the same column that kept their 0 and so
// show the true length, or a format that always starts with 0: Philippine
// mobiles (09…), UK phone numbers (07…, 01…, 02…) and US ZIP codes.
func leadingZeros(header string, vals []string) *Finding {
	nonEmpty, digits := 0, 0
	keptLen := map[int]int{}     // lengths of the values that kept a leading 0
	keptHead := map[string]int{} // and their first two digits
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		nonEmpty++
		if !allDigits(v) {
			continue
		}
		digits++
		if v[0] == '0' && len(v) > 1 {
			keptLen[len(v)]++
			keptHead[v[:2]]++
		}
	}
	if digits == 0 || digits*10 < nonEmpty*8 {
		return nil // not a column of numbers
	}
	thing := noun(header)
	length, head := 0, ""
	if n, kept := top(keptLen); kept >= 2 {
		length = n
		if h, same := top(keptHead); same == kept {
			head = h // every survivor starts the same way, so the damaged ones must too
		}
	} else if length, head = knownFormat(thing, vals); length == 0 {
		return nil
	}
	var d damage
	for i, v := range vals {
		v = strings.TrimSpace(v)
		if !allDigits(v) || v[0] == '0' || len(v) >= length || length-len(v) > 2 {
			continue
		}
		fixed := strings.Repeat("0", length-len(v)) + v
		if strings.HasPrefix(fixed, head) {
			d.add(i, v, fixed)
		}
	}
	if len(d.rows) == 0 {
		return nil
	}
	n, ex := len(d.rows), d.ex[0]
	return d.finding("leading-zeros", Repairable,
		fmt.Sprintf("%s lost %s leading 0", count(n, thing), their(n)),
		fmt.Sprintf("%s should be %s. A spreadsheet took it for a number and dropped the 0 at the front.", ex.Found, ex.Fixed),
		"Repair puts the 0 back. Next time, import this column as text.")
}

// knownFormat recognises a column whose zeros are all gone from a format
// that always starts with one. It returns the full length and how the full
// number starts.
func knownFormat(thing string, vals []string) (length int, head string) {
	total, ph, ukMobile, ukLand, short, five := 0, 0, 0, 0, 0, 0
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if !allDigits(v) {
			continue
		}
		total++
		switch {
		case len(v) == 10 && v[0] == '9':
			ph++
		case len(v) == 10 && v[0] == '7':
			ukMobile++
		case len(v) == 10 && (v[0] == '1' || v[0] == '2'):
			ukLand++
		case len(v) == 3 || len(v) == 4:
			short++
		case len(v) == 5:
			five++
		}
	}
	most := func(n int) bool { return total > 0 && n*10 >= total*6 }
	switch {
	case thing == "phone numbers" && most(ph):
		return 11, "09" // Philippine mobile: 0917 123 4567
	case thing == "phone numbers" && most(ukMobile):
		return 11, "07" // UK mobile: 07700 900123
	case thing == "phone numbers" && most(ukLand):
		return 11, "0" // UK landline: 020 7946 0018
	case thing == "ZIP codes" && short > 0 && five > 0:
		return 5, "0" // US ZIP: 02134
	}
	return 0, ""
}

// top returns the most common key and how often it appears.
func top[K comparable](counts map[K]int) (best K, n int) {
	for k, c := range counts {
		if c > n || c == n && less(k, best) {
			best, n = k, c
		}
	}
	return best, n
}

// less breaks ties so that top gives the same answer every time.
func less[K comparable](a, b K) bool { return fmt.Sprint(a) < fmt.Sprint(b) }

var sciNumber = regexp.MustCompile(`^[+-]?[0-9](\.[0-9]+)?[eE]\+?([0-9]{1,3})$`)

// numberish are the column kinds where a long number is an identifier, so
// that scientific notation can only be damage.
var numberish = map[string]bool{"phone numbers": true, "card numbers": true, "tracking numbers": true,
	"order numbers": true, "account numbers": true, "IDs": true, "ZIP codes": true, "postal codes": true}

// scientific finds long numbers that a spreadsheet wrote out in scientific
// notation, such as 4.11111E+15, keeping only the first few digits.
func scientific(header string, vals []string) *Finding {
	var d damage
	longIntact := 0
	for i, v := range vals {
		v = strings.TrimSpace(v)
		if allDigits(v) && len(v) >= 10 {
			longIntact++
		}
		m := sciNumber.FindStringSubmatch(v)
		if m == nil {
			continue
		}
		if exp, _ := strconv.Atoi(m[2]); exp >= 9 {
			d.add(i, v, "")
		}
	}
	thing := noun(header)
	if len(d.rows) == 0 || !numberish[thing] && longIntact == 0 {
		return nil // big numbers in scientific notation can be real measurements
	}
	if thing == "values" {
		thing = "numbers"
	}
	n, ex := len(d.rows), d.ex[0]
	return d.finding("scientific", Lost,
		fmt.Sprintf("%s %s turned into scientific notation, like %s", count(n, thing), be(n), ex.Found),
		fmt.Sprintf("%s keeps only the first %d digits of the real number. The rest are gone.", ex.Found, mantissaDigits(ex.Found)),
		"Get these again from the system that made the file. Next time, import this column as text.")
}

// mantissaDigits counts the digits before the E in 4.11111E+15.
func mantissaDigits(s string) int {
	m, _, _ := strings.Cut(strings.ToUpper(s), "E")
	return strings.Count(m, "") - 1 - strings.Count(m, ".") - strings.Count(m, "-") - strings.Count(m, "+")
}

// cutDigits finds numbers longer than 15 digits whose ends turned into
// zeros: a spreadsheet keeps only 15 significant digits of a number, and it
// cuts every long number in a column, not some of them.
func cutDigits(header string, vals []string) *Finding {
	var d damage
	intact, longest := 0, 0
	for i, v := range vals {
		v = strings.TrimSpace(v)
		if !allDigits(v) || len(v) < 16 {
			continue
		}
		if strings.Trim(v[15:], "0") == "" {
			d.add(i, v, "")
			longest = max(longest, len(v))
		} else {
			intact++
		}
	}
	n := len(d.rows)
	if n == 0 || intact > 0 || n < 3 && longest < 17 {
		return nil // a 16-digit number ends in 0 one time in ten anyway
	}
	thing := noun(header)
	if thing == "values" {
		thing = "numbers"
	}
	ex := d.ex[0]
	return d.finding("cut-digits", Lost,
		fmt.Sprintf("%s lost %s last digits", count(n, thing), their(n)),
		fmt.Sprintf("%s ends in zeros because spreadsheets keep only the first 15 digits of a number.", ex.Found),
		"Get these again from the system that made the file. Next time, import this column as text.")
}
