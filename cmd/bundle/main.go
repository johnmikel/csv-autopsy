// Command bundle makes the self-contained web page: it inlines Go's
// wasm_exec.js, the compiled checker and the sample file into web/index.html.
// The page is written as a body fragment, ready to publish; preview.html wraps
// it in a full document for opening locally.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const skeleton = `<!doctype html><html lang="en"><head><meta charset="utf-8">` +
	`<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">` +
	`<style>:root{color-scheme:light;padding-top:env(safe-area-inset-top);padding-bottom:env(safe-area-inset-bottom)}` +
	`body{margin:0;font:14px system-ui,sans-serif}img{max-width:100%}[hidden]{display:none!important}</style></head><body>`

func main() {
	page := flag.String("page", "web/index.html", "the page template")
	glue := flag.String("glue", "", "Go's wasm_exec.js, from $(go env GOROOT)/lib/wasm")
	wasm := flag.String("wasm", "dist/csv-autopsy.wasm", "the compiled checker")
	sample := flag.String("sample", "testdata/orders-damaged.csv", "the sample file the page opens with")
	out := flag.String("o", "dist/index.html", "where to write the page")
	flag.Parse()
	html, js, bin, csv := read(*page), read(*glue), read(*wasm), read(*sample)
	quoted, _ := json.Marshal(csv)
	html = fill(html, "<!--WASM_EXEC-->", "<script>\n"+js+"</script>")
	html = fill(html, "__WASM_BASE64__", base64.StdEncoding.EncodeToString([]byte(bin)))
	html = fill(html, `"__SAMPLE_CSV__"`, string(quoted))
	write(*out, html)
	write(filepath.Join(filepath.Dir(*out), "preview.html"), skeleton+html+"</body></html>")
	log.Printf("%s: %.1f MB (checker %.1f MB)", *out, float64(len(html))/1e6, float64(len(bin))/1e6)
}

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	return string(b)
}

func write(path, s string) {
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		log.Fatal(err)
	}
}

// fill replaces the one placeholder in page with value.
func fill(page, placeholder, value string) string {
	if strings.Count(page, placeholder) != 1 {
		log.Fatalf("the page needs exactly one %s", placeholder)
	}
	return strings.Replace(page, placeholder, value, 1)
}
