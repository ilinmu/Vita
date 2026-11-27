package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/ilinmu/vita/internal/app"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/syncer"
)

func resolveDataDir() string {
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil && dir != "" {
		return filepath.Join(dir, "vita")
	}
	if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
		return filepath.Join(cfgDir, "vita")
	}
	return "./vita-data"
}

func main() {
	var state *app.App

	mygo.App.WhenReady(func() {
		store, err := localstore.Open(resolveDataDir())
		if err != nil {
			log.Fatalf("open vita local store: %v", err)
		}
		cfg := store.Config()
		client := syncer.NewClient(cfg.LANURL, cfg.PublicURL)
		state = app.New(store, client)
		mygo.KeepState(state)

		win := mygo.NewWindow(mygo.WindowOptions{
			Title:   "Vita",
			Width:   430,
			Height:  860,
			Content: ui.View(state.View),
		})
		state.Win = win
		state.TriggerSync(nil, true)
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
