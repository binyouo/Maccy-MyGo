// Command maccy is a cross-platform clipboard manager in the spirit of Maccy,
// built with the MyGo desktop framework for Go.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
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

//go:embed mygo.json
var appManifest []byte

func main() {
	var metadata struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(appManifest, &metadata); err != nil {
		log.Fatal(err)
	}
	if metadata.Version == "" {
		log.Fatal("missing version in mygo.json")
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println(metadata.Version)
		return
	}
	name := "Maccy MyGo"
	if n := os.Getenv("MACCY_NAME"); n != "" {
		name = n
	}
	mygo.App.SetName(name)
	mygo.App.SetVersion(metadata.Version)

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
