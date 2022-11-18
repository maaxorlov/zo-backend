package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"zo-backend/server"
)

func main() {
	port, router, err := server.NewRouter()
	if err != nil {
		log.Fatal(err)
	}

	// Start the server
	go func() {
		log.Println("the server is active")
		err := http.ListenAndServe(":"+port, router)
		if err != nil {
			log.Println("the server is inactive due to an error")
			log.Fatal(fmt.Errorf("listening server error: %+v\n", err))
		}
	}()

	//exit the program on ps kill, an interrupt and Ctrl+C...
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGQUIT, os.Kill, syscall.SIGTERM)
	<-exit
	log.Println("the server is inactive")
}
