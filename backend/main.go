package main

import (
	"fmt"
	"log"
	"os"

	"zui/cli"
	"zui/router"
)

func main() {
	if handled, err := cli.Run(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port

	r := router.RegisterRouter()
	log.Printf("z-ui backend listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server start failed: %v", err)
	}
}
