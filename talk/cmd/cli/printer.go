package main

import (
	"fmt"
	"os"
)

// Printer abstracts terminal output for testability.
type Printer interface {
	Printf(format string, args ...any)
	Println(args ...any)
	Errorf(format string, args ...any)
}

// stdPrinter writes to os.Stdout and os.Stderr.
type stdPrinter struct{}

func (stdPrinter) Printf(format string, args ...any) { fmt.Printf(format, args...) }
func (stdPrinter) Println(args ...any)               { fmt.Println(args...) }
func (stdPrinter) Errorf(format string, args ...any) { fmt.Fprintf(os.Stderr, format, args...) }
