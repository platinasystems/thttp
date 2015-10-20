package main

import (
	"flag"
	"fmt"
	"github.com/platinasystems/wget"
	"log"
	"os"
)

func main() {
	w := &wget.Config{}

	fs := flag.NewFlagSet("wget", flag.ExitOnError)

	fs.BoolVar(&w.Verbose, "verbose", false, "Verbose")
	fs.BoolVar(&w.Stdout, "stdout", true, "Fetch file and send to standard output")
	fs.StringVar(&w.ServePath, "serve", "", "Directory to serve files from as file server")
	fs.StringVar(&w.ServePort, "port", "9090", "TCP port for file server")
	fs.Parse(os.Args[1:])

	w.GetPaths = fs.Args()
	if len(w.GetPaths) == 0 && w.ServePath == "" {
		fmt.Printf("Usage: wget OPTIONS FILES...\n")
		fs.PrintDefaults()
		os.Exit(1)
	}

	err := w.Run()
	if err != nil {
		log.Fatal(err)
	}
}
