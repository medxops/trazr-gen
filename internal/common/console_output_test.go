package common

import (
	"bytes"
	"os"
	"testing"
)

func TestConsoleOutput_Println(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	c := ConsoleOutput{}
	c.Println("hello", "world")
	w.Close()
	os.Stdout = old
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	if got := buf.String(); got == "" {
		t.Error("Println did not write to stdout")
	}
}

func TestConsoleOutput_Printf(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	c := ConsoleOutput{}
	c.Printf("%s %d", "number", 42)
	w.Close()
	os.Stdout = old
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	if got := buf.String(); got == "" {
		t.Error("Printf did not write to stdout")
	}
}

func TestConsoleOutput_Errorln(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	c := ConsoleOutput{}
	c.Errorln("error message")
	w.Close()
	os.Stderr = old
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	if got := buf.String(); got == "" {
		t.Error("Errorln did not write to stderr")
	}
}

func TestConsoleOutput_Successln(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	c := ConsoleOutput{}
	c.Successln("success message")
	w.Close()
	os.Stdout = old
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	if got := buf.String(); got == "" {
		t.Error("Successln did not write to stdout")
	}
}

func TestConsoleOutput_Warningln(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	c := ConsoleOutput{}
	c.Warningln("warning message")
	w.Close()
	os.Stdout = old
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	if got := buf.String(); got == "" {
		t.Error("Warningln did not write to stdout")
	}
}

func TestNewConsoleOutput(t *testing.T) {
	out := NewConsoleOutput()
	if out == nil {
		t.Error("NewConsoleOutput returned nil")
	}
}
