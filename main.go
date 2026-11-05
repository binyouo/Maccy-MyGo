// Command maccy is a cross-platform clipboard manager in the spirit of Maccy,
// built with the MyGo desktop framework for Go.
package main

import (
	_ "embed"
	"log"
	"os"

	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/plugins/automation"
	"github.com/egoist/mygo/plugins/updater/native"

	"github.com/binyouo/Maccy-MyGo/internal/app"
	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

//go:embed resources/icon.png
var appIconPNG []byte

func main() {
	name := "Maccy MyGo"
	if n := os.Getenv("MACCY_NAME"); n != "" {
		name = n
	}
	mygo.App.SetName(name)
	mygo.App.SetVersion("1.0.0")

	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load(dir + "/config.json")
	if err != nil {
		log.Printf("maccy: config: %v", err)
	}
	store, err := model.Open(dir)
	if err != nil {
		log.Fatal(err)
	}

	a := app.New(cfg, store)
	a.SetAboutIcon(appIconPNG)

	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.OnSecondInstance(func(args []string, workingDir string) {
		a.TogglePopup()
	})

	mygo.Use(native.Plugin)

	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(a.Start)

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
