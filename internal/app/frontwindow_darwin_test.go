package app

import (
	"fmt"
	"testing"
)

func TestFrontmostWindowRect(t *testing.T) {
	if err := loadWindowSyms(); err != nil {
		t.Fatalf("symbols: %v", err)
	}
	pid := frontmostPID()
	fmt.Printf("frontmost pid=%d\n", pid)
	r, ok := frontmostWindowRect()
	fmt.Printf("frontmost=%v ok=%v\n", r, ok)
}
