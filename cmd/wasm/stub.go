//go:build !(js && wasm)

package main

import "fmt"

// The browser build is main.go; this stub keeps go build ./... and go vet ./... happy.
func main() { fmt.Println("build with GOOS=js GOARCH=wasm, or run make page") }
