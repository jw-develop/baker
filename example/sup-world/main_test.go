package main

import "testing"

func TestSup(t *testing.T) {
	got := Sup()
	want := "Sup, World!"
	if got != want {
		t.Errorf("Sup() = %q, want %q", got, want)
	}
}
