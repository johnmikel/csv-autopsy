package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDamagedFile(t *testing.T) {
	repaired := filepath.Join(t.TempDir(), "repaired.csv")
	var out, errOut bytes.Buffer
	if code := cli([]string{"-fix", repaired, "testdata/orders-damaged.csv"}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1 for a damaged file\n%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Found 7 problems.") || !strings.Contains(errOut.String(), "cells repaired") {
		t.Errorf("unexpected output:\n%s\n%s", out.String(), errOut.String())
	}
	// The repaired file has nothing left that can be repaired.
	out.Reset()
	cli([]string{"-json", repaired}, &out, &errOut)
	var report struct{ Repairable int }
	if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Repairable != 0 {
		t.Errorf("the repaired file still has %d repairable problems (%v):\n%s", report.Repairable, err, out.String())
	}
}

func TestCleanFile(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := cli([]string{"testdata/orders-original.csv"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, want 0 for a clean file\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "No damage found.") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}

func TestRefusesToOverwriteItsInput(t *testing.T) {
	src, _ := os.ReadFile("testdata/orders-damaged.csv")
	path := filepath.Join(t.TempDir(), "orders.csv")
	os.WriteFile(path, src, 0o644)
	var out, errOut bytes.Buffer
	if code := cli([]string{"-fix", path, path}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if now, _ := os.ReadFile(path); !bytes.Equal(now, src) {
		t.Fatal("the input file was changed")
	}
}
