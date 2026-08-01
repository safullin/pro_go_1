package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	root := prepareTestModule(t)

	if err := generate(root); err != nil {
		t.Fatalf("generate: %v", err)
	}

	sampleGenerated := filepath.Join(root, "sample", generatedName)
	otherGenerated := filepath.Join(root, "other", generatedName)
	first, err := os.ReadFile(sampleGenerated)
	if err != nil {
		t.Fatalf("read sample generated file: %v", err)
	}
	if _, err := os.Stat(otherGenerated); err != nil {
		t.Fatalf("stat other generated file: %v", err)
	}

	if err := generate(root); err != nil {
		t.Fatalf("generate second time: %v", err)
	}
	second, err := os.ReadFile(sampleGenerated)
	if err != nil {
		t.Fatalf("read generated file after second run: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generated file changed after repeated run")
	}
}

func prepareTestModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module example.com/resettest\n\ngo 1.24.0\n")
	writeTestFile(t, root, "sample/model.go", sampleModel)
	writeTestFile(t, root, "sample/model_test.go", sampleTest)
	writeTestFile(t, root, "other/model.go", otherModel)
	writeTestFile(t, root, "other/model_test.go", otherTest)
	return root
}

func writeTestFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const sampleModel = `package sample

// generate:reset
type Subject struct {
	Number         int
	Text           string
	Enabled        bool
	Numbers        []int
	Lookup         map[string]string
	NumberPointer  *int
	SlicePointer   *[]int
	MapPointer     *map[string]int
	NestedPointer  **int
	Child          Child
	ChildPointer   *Child
	Plain          Plain
}

type Child struct {
	Value       int
	ResetCalled bool
}

func (child *Child) Reset() {
	child.Value = 0
	child.ResetCalled = true
}

type Plain struct {
	Value int
}
`

const sampleTest = `package sample

import "testing"

func TestSubjectReset(t *testing.T) {
	number := 7
	numbers := make([]int, 2, 8)
	lookup := map[string]int{"key": 1}
	nestedValue := 9
	nestedPointer := &nestedValue
	child := &Child{Value: 5}
	value := &Subject{
		Number: 4,
		Text: "text",
		Enabled: true,
		Numbers: make([]int, 3, 10),
		Lookup: map[string]string{"key": "value"},
		NumberPointer: &number,
		SlicePointer: &numbers,
		MapPointer: &lookup,
		NestedPointer: &nestedPointer,
		Child: Child{Value: 3},
		ChildPointer: child,
		Plain: Plain{Value: 6},
	}

	numberPointer := value.NumberPointer
	slicePointer := value.SlicePointer
	mapPointer := value.MapPointer
	nestedOuter := value.NestedPointer
	nestedInner := *value.NestedPointer
	childPointer := value.ChildPointer
	value.Reset()

	if value.Number != 0 || value.Text != "" || value.Enabled {
		t.Fatal("primitive fields were not reset")
	}
	if value.NumberPointer != numberPointer || *value.NumberPointer != 0 {
		t.Fatal("number pointer was not preserved and reset")
	}
	if value.SlicePointer != slicePointer || len(*value.SlicePointer) != 0 || cap(*value.SlicePointer) != 8 {
		t.Fatal("slice pointer was not preserved and truncated")
	}
	if value.MapPointer != mapPointer || len(*value.MapPointer) != 0 {
		t.Fatal("map pointer was not preserved and cleared")
	}
	if value.NestedPointer != nestedOuter || *value.NestedPointer != nestedInner || **value.NestedPointer != 0 {
		t.Fatal("nested pointers were not preserved and reset")
	}
	if len(value.Numbers) != 0 || cap(value.Numbers) != 10 || value.Numbers == nil {
		t.Fatal("slice was not preserved and truncated")
	}
	if len(value.Lookup) != 0 || value.Lookup == nil {
		t.Fatal("map was not preserved and cleared")
	}
	if !value.Child.ResetCalled || value.Child.Value != 0 {
		t.Fatal("child Reset was not called")
	}
	if value.ChildPointer != childPointer || !value.ChildPointer.ResetCalled || value.ChildPointer.Value != 0 {
		t.Fatal("child pointer Reset was not called")
	}
	if value.Plain.Value != 0 {
		t.Fatal("plain nested struct was not reset")
	}
}
`

const otherModel = `package other

// generate:reset
type Other struct {
	Values []string
}
`

const otherTest = `package other

import "testing"

func TestOtherReset(t *testing.T) {
	value := &Other{Values: make([]string, 2, 5)}
	value.Reset()
	if len(value.Values) != 0 || cap(value.Values) != 5 {
		t.Fatal("values were not truncated")
	}
}
`
