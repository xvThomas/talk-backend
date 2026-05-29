package main

// ANSI colour helpers — no external dependency required.
const (
	reset       = "\033[0m"
	bold        = "\033[1m"
	dim         = "\033[2m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
)

func cyan(s string) string      { return colorCyan + s + reset }
func green(s string) string     { return colorGreen + s + reset }
func yellow(s string) string    { return colorYellow + s + reset }
func red(s string) string       { return colorRed + s + reset }
func faint(s string) string     { return dim + s + reset }
func emphasize(s string) string { return bold + s + reset }
