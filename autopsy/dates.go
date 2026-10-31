package autopsy

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	slashDate = regexp.MustCompile(`^(\d{1,2})([/.-])(\d{1,2})([/.-])(\d{4}|\d{2})((?:[ T].*)?)$`)
	isoDate   = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})((?:[ T].*)?)$`)
)

// serialDates finds dates that became plain numbers, which is how
// spreadsheets store them: 45231 is 31 October 2023.
func serialDates(header string, vals []string) *Finding {
	written := 0
	var serials []int
	for i, v := range vals {
		v = strings.TrimSpace(v)
		switch {
		case slashDate.MatchString(v), isoDate.MatchString(v):
			written++
		case isSerial(v):
			serials = append(serials, i)
		}
	}
	if len(serials) == 0 || noun(header) != "dates" && written == 0 {
		return nil
	}
	var d damage
	for _, i := range serials {
		v := strings.TrimSpace(vals[i])
		d.add(i, v, serialText(v))
	}
	n, ex := len(d.rows), d.ex[0]
	return d.finding("serial-dates", Repairable,
		fmt.Sprintf("%s turned into numbers like %s", count(n, "dates"), ex.Found),
		fmt.Sprintf("%s is how a spreadsheet stores %s. The date was saved as that number.", ex.Found, spoken(ex.Fixed)),
		"Repair turns them back into dates, written as YYYY-MM-DD.")
}

// isSerial reports whether v looks like a spreadsheet day number between 1954 and 2064.
func isSerial(v string) bool {
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && !strings.ContainsAny(v, "eE+-") && f >= 20000 && f < 60000
}

// serialText turns a day number into YYYY-MM-DD, with the time if there is one.
func serialText(v string) string {
	f, _ := strconv.ParseFloat(v, 64)
	days := math.Floor(f)
	t := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(days))
	t = t.Add(time.Duration(math.Round((f-days)*86400)) * time.Second)
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02 15:04")
}

// spoken writes 2023-10-31 as 31 October 2023.
func spoken(iso string) string {
	t, err := time.Parse("2006-01-02", iso[:min(10, len(iso))])
	if err != nil {
		return iso
	}
	return t.Format("2 January 2006") + iso[min(10, len(iso)):]
}

// written is a date as it appears in one cell.
type written struct {
	i      int
	raw    string
	a, b   int // the first two numbers of 3/4/2026; for 2026-04-03, the day and the month
	year   int
	iso    bool
	style  string // "padded" for 03/04, "bare" for 3/4, "" when it cannot tell
	suffix string // whatever follows the date, such as a time
}

// mixedDates finds dates that a spreadsheet read the wrong way round. When a
// spreadsheet that reads dates month first opens day-first dates, it leaves
// 13/04/2026 alone, since there is no 13th month, but takes 03/04/2026 for
// 4 March. Saved again, those come back rewritten as 3/4/2026, or turned
// into 2026-03-04 with the day and month swapped.
func mixedDates(header string, vals []string) *Finding {
	var ds []written
	dayFirst, monthFirst := 0, 0
	for i, v := range vals {
		v = strings.TrimSpace(v)
		if m := isoDate.FindStringSubmatch(v); m != nil {
			ds = append(ds, written{i: i, raw: v, a: atoi(m[3]), b: atoi(m[2]), year: atoi(m[1]), iso: true, suffix: m[4]})
			continue
		}
		m := slashDate.FindStringSubmatch(v)
		if m == nil || m[2] != m[4] {
			continue
		}
		w := written{i: i, raw: v, a: atoi(m[1]), b: atoi(m[3]), year: fullYear(m[5]), style: style(m[1], m[3]), suffix: m[6]}
		if w.a == 0 || w.b == 0 || w.a > 31 || w.b > 31 {
			continue
		}
		if w.a > 12 {
			dayFirst++
		}
		if w.b > 12 {
			monthFirst++
		}
		ds = append(ds, w)
	}
	switch {
	case dayFirst > 0 && monthFirst > 0:
		return mixedOrder(ds, dayFirst, monthFirst)
	case dayFirst == 0 && monthFirst == 0:
		return nil // nothing shows which way round these dates are
	}
	order, other := "day first", "month first"
	sure := func(w written) bool { return !w.iso && (dayFirst > 0 && w.a > 12 || monthFirst > 0 && w.b > 12) }
	if monthFirst > 0 {
		order, other = other, order
	}

	// How the dates that can only be read one way are written is how the
	// file was written. Ambiguous dates written another way were rewritten.
	styles := map[string]int{}
	var example written
	for _, w := range ds {
		if sure(w) {
			if w.style != "" {
				styles[w.style]++
			}
			if example.raw == "" {
				example = w
			}
		}
	}
	original, _ := top(styles)
	var rewritten, swapped []written
	isos := 0
	for _, w := range ds {
		ambiguous := w.a <= 12 && w.b <= 12
		switch {
		case w.iso:
			isos++
			if ambiguous {
				swapped = append(swapped, w)
			}
		case ambiguous && original != "" && w.style != "" && w.style != original:
			rewritten = append(rewritten, w)
		}
	}
	if len(swapped) < 3 || len(swapped) < isos {
		swapped = nil // a few ISO dates that could be either way are not enough to go on
	}
	if len(styles) > 1 {
		rewritten = nil // the file itself was not consistent, so a different style proves nothing
	}
	if len(rewritten) == 0 && len(swapped) == 0 {
		return nil
	}

	// Repair writes every date in the column as YYYY-MM-DD, the right way round.
	var d damage
	damaged := map[int]bool{}
	for _, w := range append(rewritten, swapped...) {
		damaged[w.i] = true
	}
	fixes := map[int]string{}
	for _, w := range ds {
		day, month := w.a, w.b
		switch {
		case w.iso && !damaged[w.i]:
			continue // already unambiguous and right
		case w.iso:
			day, month = w.b, w.a
		case monthFirst > 0:
			day, month = w.b, w.a
		}
		if fixed, ok := isoText(w.year, month, day); ok {
			fixes[w.i] = fixed + w.suffix
		}
	}
	all := append(rewritten, swapped...)
	sort.Slice(all, func(i, j int) bool { return all[i].i < all[j].i })
	for _, w := range all {
		d.add(w.i, w.raw, fixes[w.i])
	}
	d.fixes = fixes
	n, ex := len(all), d.ex[0]
	advice := "Repair writes every date in this column as YYYY-MM-DD, which no program can misread."
	if len(swapped) > 0 {
		return d.finding("swapped-dates", Repairable,
			fmt.Sprintf("%s %s the day and month swapped", count(n, "dates"), pick(n, "has", "have")),
			fmt.Sprintf("Most dates here are %s, like %s. These were rewritten by a program that read them %s, so %s should be %s.",
				order, example.raw, other, ex.Found, ex.Fixed),
			advice)
	}
	return d.finding("rewritten-dates", Repairable,
		fmt.Sprintf("%s %s rewritten by a spreadsheet that reads dates %s", count(n, "dates"), be(n), other),
		fmt.Sprintf("Most dates here are %s and written like %s. These look like %s instead. They still mean the right date when read %s, but any program that reads them %s gets the day and month the wrong way round.",
			order, example.raw, ex.Found, order, other),
		advice)
}

// mixedOrder reports a column holding both day-first and month-first dates,
// which no rule can sort out.
func mixedOrder(ds []written, dayFirst, monthFirst int) *Finding {
	minority := func(w written) bool { return w.b > 12 }
	if monthFirst > dayFirst {
		minority = func(w written) bool { return w.a > 12 }
	}
	var d damage
	for _, w := range ds {
		if !w.iso && minority(w) {
			d.add(w.i, w.raw, "")
		}
	}
	return d.finding("mixed-dates", Lost,
		fmt.Sprintf("This column has %s written month first and %s day first", count(monthFirst, "dates"), thousands(dayFirst)),
		"13/04/2026 can only be day first and 04/13/2026 only month first. Dates such as 03/04/2026 could be either, so there is no way to tell them apart.",
		"Find out which system wrote which rows, or ask for the file again with dates written as YYYY-MM-DD.")
}

// style says whether a date's day and month are padded with zeros (03/04),
// bare (3/4), or cannot tell (12/11).
func style(a, b string) string {
	padded, bare := false, false
	for _, p := range []string{a, b} {
		switch {
		case len(p) == 2 && p[0] == '0':
			padded = true
		case len(p) == 1:
			bare = true
		}
	}
	switch {
	case padded && !bare:
		return "padded"
	case bare && !padded:
		return "bare"
	}
	return ""
}

func isoText(year, month, day int) (string, bool) {
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if month < 1 || month > 12 || t.Day() != day {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

func fullYear(y string) int {
	n := atoi(y)
	switch {
	case len(y) == 4:
		return n
	case n < 70:
		return 2000 + n
	}
	return 1900 + n
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

var (
	dayMonth  = regexp.MustCompile(`(?i)^\d{1,2}-(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)$`)
	monthYear = regexp.MustCompile(`(?i)^(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)-\d{2}$`)
)

// codeWords are column names whose values are ranges, sizes or codes, never dates.
var codeWords = []string{"size", "range", "age", "gene", "symbol", "code", "score", "grade", "ratio", "version", "sku", "fraction"}

// textDates finds text that a spreadsheet took for a date: a range such as
// 3-4 becomes 3-Apr, and a gene name such as SEPT2 becomes 2-Sep.
func textDates(header string, vals []string) *Finding {
	if noun(header) == "dates" {
		return nil // in a date column, 3-Apr may well be a date
	}
	others := 0
	var d damage
	for i, v := range vals {
		v = strings.TrimSpace(v)
		switch {
		case v == "":
		case dayMonth.MatchString(v) || monthYear.MatchString(v):
			d.add(i, v, "")
		default:
			others++
		}
	}
	h := strings.ToLower(header)
	codes := false
	for _, w := range codeWords {
		codes = codes || strings.Contains(h, w)
	}
	if len(d.rows) == 0 || others == 0 && !codes {
		return nil
	}
	n, ex := len(d.rows), d.ex[0]
	thing := noun(header)
	return d.finding("text-dates", Lost,
		fmt.Sprintf("%s turned into dates, like %s", count(n, thing), ex.Found),
		fmt.Sprintf("A spreadsheet read these as dates. %s was probably %s, or a code such as a gene name, and there is no telling which.", ex.Found, guess(ex.Found)),
		"Check them against the source. Next time, import this column as text.")
}

// guess says what a value taken for a date might have been: 02-Mar was 2-3
// or 3-2, depending on which way the spreadsheet reads dates.
func guess(v string) string {
	a, b, _ := strings.Cut(v, "-")
	if dayMonth.MatchString(v) {
		d, m := atoi(a), monthNumber(b)
		return fmt.Sprintf("the range %d-%d or %d-%d", d, m, m, d)
	}
	return fmt.Sprintf("%d-%s", monthNumber(a), b) // Apr-13 was 4-13
}

func monthNumber(name string) int {
	t, err := time.Parse("Jan", strings.ToUpper(name[:1])+strings.ToLower(name[1:3]))
	if err != nil {
		return 0
	}
	return int(t.Month())
}
