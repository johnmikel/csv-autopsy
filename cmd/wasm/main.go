//go:build js && wasm

// Command wasm runs csv-autopsy in a web page, so that a file never leaves
// the browser. It sets one global function, csvAutopsy(name, bytes), which
// takes the file's bytes as a Uint8Array and returns the report as JSON,
// with the repaired file as text.
package main

import (
	"encoding/json"
	"strings"
	"syscall/js"

	"github.com/johnmikel/csv-autopsy/autopsy"
)

type result struct {
	autopsy.Report
	Checks   string `json:"checks"`
	Repaired string `json:"repaired"`
	Changed  int    `json:"changed"`
	Error    string `json:"error,omitempty"`
}

func examine(_ js.Value, args []js.Value) any {
	name := args[0].String()
	data := make([]byte, args[1].Length())
	js.CopyBytesToGo(data, args[1])
	out := result{Checks: autopsy.Checks}
	t, err := autopsy.Read(data)
	if err != nil {
		out.Error = err.Error()
	} else {
		findings := autopsy.Examine(t)
		out.Report = autopsy.NewReport(name, t, findings)
		repaired, changed := autopsy.Repair(t, findings)
		var b strings.Builder
		repaired.Write(&b)
		out.Repaired, out.Changed = b.String(), changed
	}
	j, _ := json.Marshal(out)
	return string(j)
}

func main() {
	js.Global().Set("csvAutopsy", js.FuncOf(examine))
	select {}
}
