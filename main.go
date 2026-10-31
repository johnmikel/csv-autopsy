// Command csv-autopsy checks a CSV file for the damage spreadsheet programs do
// when someone opens it and saves it again, says what can be undone, and
// undoes it.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/johnmikel/csv-autopsy/autopsy"
)

const usage = `csv-autopsy checks a CSV file for the damage spreadsheet programs do when
someone opens it and saves it again, says what can be undone, and undoes it.

Usage:
  csv-autopsy [-fix repaired.csv] [-json] file.csv

It exits with status 1 when it finds damage, so it can guard a data pipeline.

Flags:`

func main() { os.Exit(cli(os.Args[1:], os.Stdout, os.Stderr)) }

func cli(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("csv-autopsy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fix := fs.String("fix", "", "write a repaired copy of the file to `path`")
	asJSON := fs.Bool("json", false, "print the findings as JSON")
	fs.Usage = func() { fmt.Fprintln(stderr, usage); fs.PrintDefaults() }
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	path := fs.Arg(0)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, "csv-autopsy:", err)
		return 2
	}
	t, err := autopsy.Read(data)
	if err != nil {
		fmt.Fprintf(stderr, "csv-autopsy: %s: %v\n", path, err)
		return 2
	}
	findings := autopsy.Examine(t)
	report := autopsy.NewReport(filepath.Base(path), t, findings)
	if *asJSON {
		report.JSON(stdout)
	} else {
		report.Text(stdout, colorful(stdout))
	}
	if *fix != "" {
		if same(path, *fix) {
			fmt.Fprintln(stderr, "csv-autopsy: -fix must name a new file, not the one being checked")
			return 2
		}
		repaired, changed := autopsy.Repair(t, findings)
		var b bytes.Buffer
		if err := repaired.Write(&b); err != nil {
			fmt.Fprintln(stderr, "csv-autopsy:", err)
			return 2
		}
		if err := os.WriteFile(*fix, b.Bytes(), 0o644); err != nil {
			fmt.Fprintln(stderr, "csv-autopsy:", err)
			return 2
		}
		fmt.Fprintf(stderr, "Wrote %s, with %d %s repaired.\n", *fix, changed, map[bool]string{true: "cell", false: "cells"}[changed == 1])
	}
	if len(findings) > 0 {
		return 1
	}
	return 0
}

// same reports whether two paths name the same file.
func same(a, b string) bool {
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

// colorful reports whether w is a terminal that wants colour.
func colorful(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
