package config

import (
	"path/filepath"
	"testing"
)

func TestIntervalChangeSubscriptions(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	a, cancelA := c.SubscribeIntervalChanges()
	b, cancelB := c.SubscribeIntervalChanges()
	defer cancelA()
	defer cancelB()
	update := func(interval float64) {
		t.Helper()
		if err := c.Update(func(c *Config) { c.ClipboardCheckInterval = interval }); err != nil {
			t.Fatal(err)
		}
	}
	read := func(ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		default:
			t.Fatal("missing interval notification")
		}
	}
	quiet := func(ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
			t.Fatal("unexpected interval notification")
		default:
		}
	}
	update(.1)
	read(a)
	update(.2) // b's pending event coalesces; both readers see the latest value.
	read(a)
	read(b)
	quiet(b)
	if c.Interval() != .2 {
		t.Fatal("notification did not expose latest interval")
	}
	cancelA()
	cancelA()
	update(.3)
	quiet(a)
	read(b)
	if err := c.Update(func(c *Config) { c.SearchMode = SearchFuzzy }); err != nil {
		t.Fatal(err)
	}
	quiet(b)
}
