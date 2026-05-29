package main

import "testing"

func TestCyan(t *testing.T) {
	got := cyan("hello")
	want := colorCyan + "hello" + reset
	if got != want {
		t.Errorf("cyan(\"hello\") = %q, want %q", got, want)
	}
}

func TestGreen(t *testing.T) {
	got := green("ok")
	want := colorGreen + "ok" + reset
	if got != want {
		t.Errorf("green(\"ok\") = %q, want %q", got, want)
	}
}

func TestYellow(t *testing.T) {
	got := yellow("warn")
	want := colorYellow + "warn" + reset
	if got != want {
		t.Errorf("yellow(\"warn\") = %q, want %q", got, want)
	}
}

func TestRed(t *testing.T) {
	got := red("err")
	want := colorRed + "err" + reset
	if got != want {
		t.Errorf("red(\"err\") = %q, want %q", got, want)
	}
}

func TestFaint(t *testing.T) {
	got := faint("dim")
	want := dim + "dim" + reset
	if got != want {
		t.Errorf("faint(\"dim\") = %q, want %q", got, want)
	}
}

func TestEmphasize(t *testing.T) {
	got := emphasize("bold")
	want := bold + "bold" + reset
	if got != want {
		t.Errorf("emphasize(\"bold\") = %q, want %q", got, want)
	}
}
