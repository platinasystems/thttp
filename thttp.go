// Copyright 2015 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style license described in the
// LICENSE file.

// Package thttp (Trivial HTTP) provides support for fetching & serving files via http.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
)

type Config struct {
	ServePath         string
	ServePort         string
	serveFilesHandler http.Handler

	GetPaths []string
	PutPath  string
	Verbose  bool
	// Output to stdout instead of file
	Stdout bool
}

func (w *Config) Get(u *url.URL) (err error) {
	res, err := http.Get(u.String())
	if err != nil {
		return
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("%s: %s", u, http.StatusText(res.StatusCode))
		return
	}
	writer := os.Stdout
	if !w.Stdout {
		outFile := path.Base(u.Path)
		writer, err = os.Create(outFile)
		if err != nil {
			err = fmt.Errorf("%s: %s", outFile, err)
			return
		}
	}
	if w.Verbose {
		len := ""
		if res.ContentLength != -1 {
			len = fmt.Sprintf(" (%d bytes)", res.ContentLength)
		}
		fmt.Fprintf(os.Stderr, "Fetching %s%s\n", u, len)
	}
	_, err = io.Copy(writer, res.Body)
	res.Body.Close()
	return
}

func (w *Config) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.serveFilesHandler.ServeHTTP(res, req)
	case "PUT", "APPEND":
		var (
			f   *os.File
			err error
		)
		path := filepath.Join(w.ServePath, req.URL.Path)
		if req.Method == "APPEND" {
			f, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		} else {
			f, err = os.Create(path)
		}
		if err != nil {
			http.Error(res, fmt.Sprintf("create fails %s", err), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		_, err = io.Copy(f, req.Body)
		if err != nil {
			http.Error(res, fmt.Sprintf("copy fails %s", err), http.StatusInternalServerError)
			return
		}
		res.WriteHeader(http.StatusOK)
	default:
		http.Error(res, fmt.Sprintf("unknown method: %s", req.Method), http.StatusBadRequest)
	}
}

func (w *Config) Run() (err error) {
	// Must either have files to fetch or a directory to serve.
	if len(w.GetPaths) == 0 && len(w.PutPath) == 0 && w.ServePath == "" {
		return fmt.Errorf("no files to fetch and no directory to serve")
	}

	var wg sync.WaitGroup
	if w.ServePath != "" {
		w.serveFilesHandler = http.FileServer(http.Dir(w.ServePath))
		http.Handle("/", w)
		var listener net.Listener
		listener, err = net.Listen("tcp", ":"+w.ServePort)
		if err != nil {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			http.Serve(listener, nil)
		}()
		if len(w.GetPaths) == 0 {
			wg.Wait()
		}
	}

	for _, path := range w.GetPaths {
		var u *url.URL
		u, err = url.Parse(path)
		if err != nil {
			return
		}
		if u.Scheme == "" {
			u.Scheme = "http"
		}
		if u.Host == "" {
			u.Host = fmt.Sprintf("localhost:%s", w.ServePort)
		}

		err = w.Get(u)
		if err != nil {
			return
		}
	}

	if len(w.PutPath) > 0 {
		c := &http.Client{}
		var (
			req      *http.Request
			res      *http.Response
			contents []byte
		)
		contents, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return
		}
		url := w.PutPath
		req, err = http.NewRequest("PUT", url, bytes.NewReader(contents))
		req.ContentLength = int64(len(contents))
		res, err = c.Do(req)
		if err != nil {
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			msg, _ := ioutil.ReadAll(res.Body)
			err = fmt.Errorf("put %s: %s", url, msg)
			return
		}
	}

	return
}

func main() {
	w := &Config{}

	fs := flag.NewFlagSet("thttp", flag.ExitOnError)

	fs.BoolVar(&w.Verbose, "verbose", false, "Verbose")
	fs.BoolVar(&w.Stdout, "stdout", true, "Fetch file and send to standard output")
	fs.StringVar(&w.ServePath, "serve", "", "Directory to serve files from as file server")
	fs.StringVar(&w.ServePort, "port", "9090", "TCP port for file server")
	fs.StringVar(&w.PutPath, "put", "", "URL to http PUT standard input")
	fs.Parse(os.Args[1:])

	w.GetPaths = fs.Args()
	if len(w.GetPaths) == 0 && len(w.PutPath) == 0 && w.ServePath == "" {
		fmt.Printf("Usage: thttp OPTIONS FILES...\n")
		fs.PrintDefaults()
		os.Exit(1)
	}

	err := w.Run()
	if err != nil {
		fmt.Println(err)
	}
}
