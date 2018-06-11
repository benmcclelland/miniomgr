package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

var fsTopMount = getenv("MINIO_MGR_PATH")

func getenv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		panic("missing required environment variable " + name)
	}
	return v
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	r, err := New(fsTopMount)
	if err != nil {
		log.Fatalln("could not init proxy: %v", err)
	}

	go func() {
		sig := <-sigs
		fmt.Println("Exiting on signal", sig.String())
		fmt.Println("Shutting down backends...")
		r.Done()
		os.Exit(0)
	}()

	http.HandleFunc("/", r.handle)
	log.Fatal(http.ListenAndServe(":8080", logRequest(http.DefaultServeMux)))
}
