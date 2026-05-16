package main

import (
	"log"

	"github.com/brian-nunez/video-to-blog-page/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
