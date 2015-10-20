// Copyright 2015 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style license described in the
// LICENSE file.

// Package wget provides support for fetching & serving files via http.
package wget

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
)

type Config struct {
	ServePath string
	ServePort string
	GetPaths  []string
	Verbose   bool
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

func (w *Config) Run() (err error) {
	// Must either have files to fetch or a directory to serve.
	if len(w.GetPaths) == 0 && w.ServePath == "" {
		return fmt.Errorf("no files to fetch and no directory to serve")
	}

	var wg sync.WaitGroup
	if w.ServePath != "" {
		http.Handle("/", http.FileServer(http.Dir(w.ServePath)))
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
	return
}
