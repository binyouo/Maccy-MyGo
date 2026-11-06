//go:build darwin

package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

// A separate process keeps AppKit on its original main thread and leaves
// ordinary clipboard tests headless. The probe writes only a unique named
// pasteboard; it never reads or replaces general clipboard content.
func TestMain(m *testing.M) {
	if os.Getenv("MACCY_NATIVE_POLLING_PROBE") == "1" {
		if err := probeNativePollingTimer(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if os.Getenv("MACCY_NATIVE_SEQUENCE_PROBE") == "1" {
		if err := probeNativeClipboardSequence(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNativeClipboardSequenceCountsIdenticalWrites(t *testing.T) {
	if os.Getenv("MACCY_NATIVE_CLIPBOARD_TEST") != "1" {
		t.Skip("set MACCY_NATIVE_CLIPBOARD_TEST=1 to run the isolated AppKit probe")
	}
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = append(os.Environ(), "MACCY_NATIVE_SEQUENCE_PROBE=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native sequence probe: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func probeNativeClipboardSequence() error {
	var result error
	mygo.App.SetName("MaccyClipboardSequenceProbe")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		defer mygo.App.Quit()
		before, available := ChangeCount()
		if !available {
			result = fmt.Errorf("NSPasteboard.changeCount is unavailable")
			return
		}
		board := objc.ID(objc.GetClass("NSPasteboard")).Send(objc.RegisterName("pasteboardWithUniqueName"))
		if board == 0 {
			result = fmt.Errorf("cannot create the isolated named pasteboard")
			return
		}
		defer board.Send(objc.RegisterName("releaseGlobally"))
		stringClass := objc.ID(objc.GetClass("NSString"))
		value := stringClass.Send(objc.RegisterName("stringWithUTF8String:"), "identical sequence fixture")
		format := stringClass.Send(objc.RegisterName("stringWithUTF8String:"), "public.utf8-plain-text")
		counts := []int64{objc.Send[int64](board, pasteboardChangeSelector)}
		for i := 0; i < 2; i++ {
			board.Send(objc.RegisterName("clearContents"))
			board.Send(objc.RegisterName("setString:forType:"), value, format)
			counts = append(counts, objc.Send[int64](board, pasteboardChangeSelector))
		}
		after, ok := ChangeCount()
		if counts[1] <= counts[0] || counts[2] <= counts[1] {
			result = fmt.Errorf("identical writes did not advance native sequence: %v", counts)
			return
		}
		if !ok || before != after {
			result = fmt.Errorf("general clipboard ownership changed during the isolated probe")
			return
		}
		fmt.Printf("named pasteboard sequence=%v; general sequence readable and unchanged\n", counts)
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	return result
}
