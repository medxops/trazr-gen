package common

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

// UserOutput defines methods for user-facing CLI output.
type UserOutput interface {
	Println(args ...any)
	Printf(format string, args ...any)
	Errorln(args ...any)
	Successln(args ...any)
	Warningln(args ...any)
}

// ConsoleOutput implements UserOutput with color support.
type ConsoleOutput struct{}

func (c ConsoleOutput) Println(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}

func (c ConsoleOutput) Printf(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format, args...)
}

func (c ConsoleOutput) Errorln(args ...any) {
	color.New(color.FgRed).Fprintln(os.Stderr, args...)
}

func (c ConsoleOutput) Successln(args ...any) {
	color.New(color.FgGreen).Fprintln(os.Stdout, args...)
}

func (c ConsoleOutput) Warningln(args ...any) {
	color.New(color.FgYellow).Fprintln(os.Stdout, args...)
}

// NewConsoleOutput returns a new ConsoleOutput instance.
func NewConsoleOutput() UserOutput {
	return ConsoleOutput{}
}
