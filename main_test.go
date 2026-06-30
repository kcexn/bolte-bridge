package main

import "testing"

// TestSmoke is a trivial always-passing test used to bootstrap CI.
func TestSmoke(t *testing.T) {
	if 1+1 != 2 {
		t.Fatal("arithmetic is broken")
	}
}
