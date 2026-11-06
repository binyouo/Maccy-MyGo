// Command idleprobe isolates idle wakeups in the MyGo runtime and hidden host.
// It has no visible windows. Capture uses the benchmark's named pasteboard only.
package main

import (
	"github.com/binyouo/Maccy-MyGo/internal/clipboard"
	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"os"
	"time"
)

func main() {
	mode := os.Args[1]
	mygo.App.SetName("MyGo Idle Probe")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		if mode == "timer" || mode == "mainpoll" || mode == "count" {
			go func() {
				for {
					time.Sleep(500 * time.Millisecond)
					if mode == "mainpoll" {
						mygo.RunOnMain(func() {})
					}
					if mode == "count" {
						clipboard.ChangeCount()
					}
				}
			}()
		}
		if mode == "hidden" {
			mygo.NewWindow(mygo.WindowOptions{Hidden: true, Width: 1, Height: 1, Content: ui.View(func(c *ui.Context) { c.Root().Background(ui.Transparent) })})
		}
		if mode == "watcher" {
			dir, err := os.MkdirTemp("", "mygo-idle-watcher-")
			if err != nil {
				panic(err)
			}
			store, err := model.Open(dir)
			if err != nil {
				panic(err)
			}
			watcher := clipboard.New(config.Default(), store)
			watcher.Start()
			mygo.App.OnWillQuit(func(*mygo.QuitEvent) { watcher.Stop(); os.RemoveAll(dir) })
		}
	})
	if err := mygo.App.Run(); err != nil {
		panic(err)
	}
}
