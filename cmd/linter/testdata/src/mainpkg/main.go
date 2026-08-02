package main

import (
	"log"
	"os"
)

func main() {
	log.Fatal("allowed")
	os.Exit(0)
	panic("boom") // want "use of built-in panic is prohibited"

	callback := func() {
		log.Fatal("stop") // want "log.Fatal may only be called"
		os.Exit(1)        // want "os.Exit may only be called"
	}
	_ = callback
}

func helper() {
	log.Fatal("stop") // want "log.Fatal may only be called"
	os.Exit(1)        // want "os.Exit may only be called"
}

type runner struct{}

func (runner) main() {
	os.Exit(1) // want "os.Exit may only be called"
}
