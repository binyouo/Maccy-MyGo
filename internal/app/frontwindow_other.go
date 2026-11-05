//go:build !darwin

package app

import "github.com/egoist/mygo"

// frontmostWindowRect has no window list to query off macOS; callers
// fall back to the cursor position.
func frontmostWindowRect() (mygo.Rectangle, bool) { return mygo.Rectangle{}, false }
