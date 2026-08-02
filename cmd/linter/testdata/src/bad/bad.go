package bad

import (
	stdlog "log"
	stdos "os"
)

func forbidden() {
	panic("boom")        // want "use of built-in panic is prohibited"
	stdlog.Fatal("stop") // want "log.Fatal may only be called"
	stdos.Exit(1)        // want "os.Exit may only be called"
}

func main() {
	stdlog.Fatal("stop") // want "log.Fatal may only be called"
	stdos.Exit(1)        // want "os.Exit may only be called"
}

func shadowedNames() {
	panic := func(string) {}
	panic("allowed")

	logValue := struct {
		Fatal func(...any)
	}{Fatal: func(...any) {}}
	logValue.Fatal("allowed")

	osValue := struct {
		Exit func(int)
	}{Exit: func(int) {}}
	osValue.Exit(0)
}
