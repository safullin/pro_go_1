//go:build integration

package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestGeneratedCode(t *testing.T) {
	root := prepareTestModule(t)
	if err := generate(root); err != nil {
		t.Fatalf("generate: %v", err)
	}

	command := exec.Command("go", "test", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "GOWORK=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("test generated code: %v\n%s", err, output)
	}
}
