//go:build ignore

// Generate writes the sample files: orders-original.csv, a shop's order
// export, and orders-damaged.csv, the same file after a round trip through a
// spreadsheet that reads dates month first and mistook its UTF-8 for
// Windows-1252. Run it with: go run testdata/generate.go
package main

import (
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	names = []string{"Niña Dela Cruz", "Carlo Peñaranda", "Tom Baker", "José Ramírez", "Priya Natarajan",
		"Zoë Hartley", "Liam O'Connor", "Ana Peñaflor", "Joy Santos", "Chloé Martin", "Ramón Villanueva",
		"Grace Okafor", "Amélie Dubois", "Sam Patel", "Søren Holm", "Maya Lewis"}
	postcodes = []string{"SW1A 1AA", "M1 1AE", "B33 8TH", "CR2 6XH", "DN55 1PT", "EC1A 1BB", "W1A 0AX", "LS1 4AP"}
	sizes     = []string{"2-3", "4-5", "6-7", "8-9", "10-11", "S", "M", "L"}
	notes     = []string{"", "", "Leave with neighbour", "Customer’s gift – wrap please", "", "Call on arrival", "Fragile – handle with care", ""}
)

var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

func main() {
	rng := rand.New(rand.NewPCG(31, 10))
	header := []string{"order_id", "customer_name", "phone", "ordered_on", "delivered_at", "postcode", "size", "amount_gbp", "notes"}
	var clean, broken [][]string
	day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for i := range 40 {
		day = day.AddDate(0, 0, 1+rng.IntN(5))
		delivered := day.AddDate(0, 0, 2+rng.IntN(3))
		id := fmt.Sprintf("%s%08d", day.Format("20060102"), 4000+rng.IntN(90000))
		phone := fmt.Sprintf("07700900%03d", rng.IntN(1000))
		if i%5 == 4 {
			phone = phone[:5] + " " + phone[5:8] + " " + phone[8:] // written with spaces, so it stays text
		}
		size := sizes[rng.IntN(len(sizes))]
		amount := fmt.Sprintf("%d.%02d", 8+rng.IntN(60), []int{0, 50, 99, 25}[rng.IntN(4)])
		row := []string{id, names[rng.IntN(len(names))], phone, day.Format("02/01/2006"), delivered.Format("2006-01-02"),
			postcodes[rng.IntN(len(postcodes))], size, amount, notes[rng.IntN(len(notes))]}
		clean = append(clean, row)
		broken = append(broken, damage(row, day, delivered, i))
	}
	write("testdata/orders-original.csv", header, clean)
	write("testdata/orders-damaged.csv", header, broken)
}

// damage does to one row what the round trip did.
func damage(row []string, ordered, delivered time.Time, i int) []string {
	out := append([]string(nil), row...)
	// Long numbers come back in scientific notation.
	f, _ := strconv.ParseFloat(row[0], 64)
	out[0] = strconv.FormatFloat(f, 'E', 5, 64)
	// UTF-8 read as Windows-1252 scrambles every accent.
	out[1], out[8] = scramble(row[1]), scramble(row[8])
	// Phone numbers made only of digits lose their 0.
	if !strings.Contains(row[2], " ") {
		out[2] = strings.TrimLeft(row[2], "0")
	}
	// Dates that could be month first (day 12 or less) are read that way and
	// written back as m/d/yyyy, which leaves their digits in place but unpadded.
	if ordered.Day() <= 12 {
		out[3] = fmt.Sprintf("%d/%d/%d", ordered.Day(), int(ordered.Month()), ordered.Year())
	}
	// Someone reformatted part of the delivery column as General.
	if i%4 == 1 {
		out[4] = strconv.Itoa(int(delivered.Sub(excelEpoch).Hours() / 24))
	}
	// Sizes such as 2-3 are taken for dates and shown as 02-Mar.
	if a, b, ok := strings.Cut(row[6], "-"); ok {
		d, _ := strconv.Atoi(a)
		m, _ := strconv.Atoi(b)
		out[6] = fmt.Sprintf("%02d-%s", d, time.Month(m).String()[:3])
	}
	// Trailing zeros of prices go, which does no harm.
	out[7] = strconv.FormatFloat(mustFloat(row[7]), 'f', -1, 64)
	return out
}

// scramble encodes s as UTF-8 and reads the bytes back as Windows-1252.
func scramble(s string) string {
	cp := map[byte]rune{0x80: '€', 0x93: '“', 0x94: '”', 0x99: '™', 0x9C: 'œ', 0x9D: 0x9D, 0x91: '‘', 0x92: '’', 0x96: '–', 0x97: '—'}
	var b strings.Builder
	for _, c := range []byte(s) {
		if r, ok := cp[c]; ok {
			b.WriteRune(r)
		} else {
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

func mustFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return f
}

func write(path string, header []string, rows [][]string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	w := csv.NewWriter(f)
	w.Write(header)
	w.WriteAll(rows)
	if err := f.Close(); err != nil {
		panic(err)
	}
}
