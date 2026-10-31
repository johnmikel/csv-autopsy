package autopsy

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func read(t *testing.T, path string) *Table {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

// kinds lists what was found, as "column: kind".
func kinds(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Column+": "+f.Kind)
	}
	return out
}

// TestSample checks the damaged sample against the original it came from.
func TestSample(t *testing.T) {
	damaged := read(t, "../testdata/orders-damaged.csv")
	original := read(t, "../testdata/orders-original.csv")

	found := Examine(damaged)
	want := []string{
		"order_id: scientific",
		"customer_name: scrambled",
		"phone: leading-zeros",
		"ordered_on: rewritten-dates",
		"delivered_at: serial-dates",
		"size: text-dates",
		"notes: scrambled",
	}
	if got := kinds(found); !reflect.DeepEqual(got, want) {
		t.Fatalf("found\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	repaired, changed := Repair(damaged, found)
	if changed == 0 {
		t.Fatal("Repair changed nothing")
	}
	col := map[string]int{}
	for i, h := range original.Header {
		col[h] = i
	}
	for i, row := range original.Rows {
		fixed := repaired.Rows[i]
		for _, name := range []string{"customer_name", "phone", "delivered_at", "notes"} {
			if fixed[col[name]] != row[col[name]] {
				t.Errorf("row %d %s: repaired %q, original %q", i+2, name, fixed[col[name]], row[col[name]])
			}
		}
		// Ordered dates come back as YYYY-MM-DD, meaning the same day as the original.
		day, _ := time.Parse("02/01/2006", row[col["ordered_on"]])
		if got := fixed[col["ordered_on"]]; got != day.Format("2006-01-02") {
			t.Errorf("row %d ordered_on: repaired %q, original %s", i+2, got, row[col["ordered_on"]])
		}
		// What cannot be undone is left as it was.
		if fixed[col["order_id"]] != damaged.Rows[i][col["order_id"]] || fixed[col["size"]] != damaged.Rows[i][col["size"]] {
			t.Errorf("row %d: Repair changed a column it cannot repair", i+2)
		}
	}
}

// TestOriginalIsClean guards against false alarms on the undamaged file.
func TestOriginalIsClean(t *testing.T) {
	if found := Examine(read(t, "../testdata/orders-original.csv")); len(found) > 0 {
		t.Fatalf("false alarms on the original file: %v", kinds(found))
	}
}

func TestDetectors(t *testing.T) {
	cases := []struct {
		name, header string
		vals         []string
		kind         string // "" means nothing should be found
		fixes        map[int]string
	}{
		{"ZIP codes, some kept their 0", "zip", []string{"02134", "2134", "10001", "01002", "1002"}, "leading-zeros",
			map[int]string{1: "02134", 4: "01002"}},
		{"Philippine mobiles, all lost their 0", "contact_number", []string{"9171234567", "9281234567", "09391234567"}, "leading-zeros",
			map[int]string{0: "09171234567", 1: "09281234567"}},
		{"UK mobiles", "Mobile", []string{"7700900123", "7700900456", "7700900789"}, "leading-zeros",
			map[int]string{0: "07700900123", 1: "07700900456", 2: "07700900789"}},
		{"quantities are not phone numbers", "qty", []string{"1", "2", "10", "05"}, "", nil},
		{"order numbers in scientific notation", "tracking_no", []string{"4.11111E+15", "1234567890123"}, "scientific", nil},
		{"measurements may be scientific", "distance_m", []string{"1.5E+10", "2.3E+11"}, "", nil},
		{"card numbers cut to 15 digits", "card_number", []string{"4111111111111110", "5500000000000000", "4012888888881880"}, "cut-digits", nil},
		{"16-digit IDs end in 0 by chance", "id", []string{"2026010500077080", "2026010900055885", "2026011200068459"}, "", nil},
		{"day numbers in a date column", "created_at", []string{"45231", "2023-10-31", "45231.5"}, "serial-dates",
			map[int]string{0: "2023-11-01", 2: "2023-11-01 12:00"}},
		{"day numbers are fine in an amount column", "amount", []string{"45231", "100"}, "", nil},
		{"ISO dates swapped by a month-first reader", "date",
			[]string{"13/04/2026", "15/04/2026", "2026-03-04", "2026-05-06", "2026-01-02"}, "swapped-dates",
			map[int]string{0: "2026-04-13", 1: "2026-04-15", 2: "2026-04-03", 3: "2026-06-05", 4: "2026-02-01"}},
		{"day-first and month-first in one column", "date", []string{"13/04/2026", "04/13/2026", "05/06/2026"}, "mixed-dates", nil},
		{"dates that are all ambiguous prove nothing", "date", []string{"03/04/2026", "05/06/2026"}, "", nil},
		{"gene names taken for dates", "gene", []string{"TP53", "1-Sep", "BRCA1"}, "text-dates", nil},
		{"3-Apr is a fine birthday", "birthday", []string{"3-Apr", "5-May"}, "", nil},
		{"scrambled names", "name", []string{"PeÃ±aflor", "José", "Ångström", "â€œhiâ€\u009d"}, "scrambled",
			map[int]string{0: "Peñaflor", 3: "“hi”"}},
		{"scrambled twice", "name", []string{"PeÃƒÂ±aflor"}, "scrambled", map[int]string{0: "Peñaflor"}},
	}
	for _, c := range cases {
		var got *Finding
		for _, detect := range detectors {
			if f := detect(c.header, c.vals); f != nil {
				if got != nil {
					t.Errorf("%s: found both %s and %s", c.name, got.Kind, f.Kind)
				}
				got = f
			}
		}
		switch {
		case c.kind == "" && got != nil:
			t.Errorf("%s: false alarm %s: %s", c.name, got.Kind, got.Title)
		case c.kind == "":
		case got == nil:
			t.Errorf("%s: found nothing, want %s", c.name, c.kind)
		case got.Kind != c.kind:
			t.Errorf("%s: found %s, want %s", c.name, got.Kind, c.kind)
		case c.fixes != nil && !reflect.DeepEqual(got.fixes, c.fixes):
			t.Errorf("%s: repairs %v, want %v", c.name, got.fixes, c.fixes)
		}
	}
}

func TestReadWindows1252(t *testing.T) {
	tbl, err := Read([]byte("name;city\nJos\xe9;Para\xf1aque\n"))
	if err != nil {
		t.Fatal(err)
	}
	if tbl.Comma != ';' || tbl.Encoding != "Windows-1252" || tbl.Rows[0][0] != "José" || tbl.Rows[0][1] != "Parañaque" {
		t.Fatalf("read %+v", tbl)
	}
	if got := kinds(Examine(tbl)); !reflect.DeepEqual(got, []string{": encoding"}) {
		t.Fatalf("found %v, want only the encoding", got)
	}
	var b strings.Builder
	repaired, _ := Repair(tbl, nil)
	repaired.Write(&b)
	if b.String() != "name;city\nJosé;Parañaque\n" {
		t.Fatalf("repaired file is %q", b.String())
	}
}

func TestRowLimit(t *testing.T) {
	tbl := &Table{Header: []string{"a"}, Rows: make([][]string, 65_535), Encoding: "UTF-8"}
	if got := kinds(Examine(tbl)); !reflect.DeepEqual(got, []string{": row-limit"}) {
		t.Fatalf("found %v, want the row limit", got)
	}
}

func TestTextReport(t *testing.T) {
	damaged := read(t, "../testdata/orders-damaged.csv")
	var b strings.Builder
	NewReport("orders-damaged.csv", damaged, Examine(damaged)).Text(&b, false)
	for _, want := range []string{
		"orders-damaged.csv: 40 rows, 9 columns",
		"Found 7 problems. 5 of them can be repaired with -fix.",
		"phone: 32 phone numbers lost their leading 0 [can be repaired]",
		"7700900939 should be 07700900939.",
		"order_id: 40 order numbers were turned into scientific notation, like 2.02601E+15 [cannot be undone]",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, b.String())
		}
	}
}
