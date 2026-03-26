package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/fogleman/primitive/web/frontend"
	"github.com/fogleman/primitive/web/server"
)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	// Strip the "dist" prefix so files are served from root
	dist, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		log.Fatal(err)
	}

	srv := server.NewServerWithFrontend(dist)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("listening on http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}
