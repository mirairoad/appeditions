// Command appeditions turns raw app screenshots into store-ready App Store and
// Google Play assets, in every language a project ships in.
//
// It is a local tool. Nothing is uploaded: the screenshots are copied into
// ~/.appeditions, the rendering happens in this process, and the files land in a
// folder next to them.
package main

import (
	"context"
	"flag"
	"log"
	"log/slog"

	"github.com/mirairoad/howl-go/core/console"

	"github.com/mirairoad/appeditions/boot"
	"github.com/mirairoad/appeditions/internal/store"
)

//go:generate go run github.com/mirairoad/howl-go/core/cmd/fsroutes -module github.com/mirairoad/appeditions/client/pages
//go:generate go run github.com/mirairoad/howl-go/core/cmd/fsapis -dir server/apis -module github.com/mirairoad/appeditions/server/apis -client client/api/api_gen.go -client-pkg apiclient

func main() {
	root := flag.String("data", store.DefaultRoot(), "where projects, screenshots and exports live")
	debug := flag.Bool("debug", false, "log at debug level")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	console.Setup(console.Options{Level: level})

	a, mux, s, err := boot.New(context.Background(), *root)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	log.Fatal(a.Listen(mux))
}
