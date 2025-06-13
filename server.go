package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/fatih/color"
	"github.com/gorilla/mux"
	"stream-widgets/services"
)

func main() {

	// Create a new router using gorilla/mux
	R := mux.NewRouter()

	// Define a handler for the root path "/"
	R.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set the Content-Type to HTML
		w.Header().Set("Content-Type", "text/html")
		// Write a simple HTML response
		fmt.Fprint(w, `
            <!DOCTYPE html>
            <html>
            <head>
                <title>Hello World</title>
            </head>
            <body>
                <h1>Hello, World!</h1>
                <p>Welcome to my minimal Go web server!</p>
            </body>
            </html>
        `)
	})

	chat.RegisterChatRoutes(R)

	fmt.Printf("Server starting on %s", color.CyanString("http://localhost:1776\n"))
	if err := http.ListenAndServe(":1776", R); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
