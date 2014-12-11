package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// waitGroup uses for counting request that is not finished yet
// help to gracfully shutdown server after all requests are done
var waitGroup sync.WaitGroup

// handleConn is used to handle each request individually.
// if it is folder, it will return a list
// if it is file, it will return its content
func handleConn(conn net.Conn) {
	// close conn and tell waitGroup decrease by one because a request is finished
	defer func() {
		conn.Close()
		waitGroup.Done()
	}()
	// count one more request
	waitGroup.Add(1)
	// Read request data from Conn and change to Request struct
	bufReader := bufio.NewReader(conn)
	req, err := http.ReadRequest(bufReader)
	if err != nil {
		log.Println(err)
		return
	}
	// Get Path from URL
	path := req.URL.Path
	// We assume to use current directory as source file server
	curDir := http.Dir(".")
	// Open file or folder following url path
	target, err := curDir.Open(path)
	if err != nil {
		log.Println(err)
		return
	}
	defer target.Close()
	targetStat, err := target.Stat()
	if err != nil {
		log.Println(err)
		return
	}
	// if directory, we shows a list of file/folder
	// if file, we return content
	if targetStat.IsDir() {
		dirs, err := ioutil.ReadDir(targetStat.Name())
		if err != nil {
			log.Println(err)
			return
		}
		for _, d := range dirs {
			name := d.Name()
			if d.IsDir() {
				name += "/"
			}
			url := url.URL{Path: name}
			fmt.Fprintf(conn, "<a href=\"%s\">%s</a>\n", url.String(), name)
		}
	} else {
		io.Copy(conn, target)
	}
}

// serverFile is function to start server and loop getting any new connection
func serveFile(listener net.Listener) {
	log.Println("Start File Server...")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			conn.Close()
			continue
		}
		go handleConn(conn)
	}
}

// closeServer is used to gracefully shutdown server by waiting for all waitGroup request to finish
func closeServer(listener net.Listener) {
	log.Println("Shutting down File Server...")
	waitGroup.Wait()
	listener.Close()
}

func main() {
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		panic(err)
	}
	// Start file server
	go serveFile(listener)
	// Handle SIGINT and SIGTERM.
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	// Spawn close server routines which wait until all requests are done
	closeServer(listener)
}
