package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"sync"
	"syscall"
)

// waitGroup uses for counting request that is not finished yet
// help to gracfully shutdown server after all requests are done
var waitGroup sync.WaitGroup

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func returnResp(conn net.Conn, statusCode string, statusText string, contentType string, content []byte) {
	fmt.Fprintf(conn, "HTTP/1.0 "+statusCode+" "+statusText+"\n")
	fmt.Fprintf(conn, "Content-Type: "+contentType+"\n")
	fmt.Fprintf(conn, "Content-Length: "+strconv.Itoa(len(content))+"\n")
	fmt.Fprintf(conn, "\n")
	conn.Write(content)
}

func getContentType(name string) string {
	match, _ := regexp.MatchString(`\w+.(jpg|JPG|JPEG|jpeg)`, name)
	if match {
		return "image/jpeg"
	}
	return "text/html"
}

// handleConn is used to handle each request individually.
// if it is folder, it will return a list
// if it is file, it will return its content
func handleConn(conn net.Conn) {
	// close conn and tell waitGroup decrease by one because a request is finished
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
			returnResp(conn, "500", "Internal Server Error", "text/plain", []byte(r.(error).Error()))
		}
		waitGroup.Done()
		conn.Close()
	}()
	// count one more request
	waitGroup.Add(1)
	// Read request data from Conn and change to Request struct
	bufReader := bufio.NewReader(conn)
	req, err := http.ReadRequest(bufReader)
	checkErr(err)
	// Get Path from URL
	path := req.URL.Path
	// We assume to use public/www directory as source file server
	publicDir := http.Dir("public/www")
	// Open file or folder following url path
	target, err := publicDir.Open(path)
	if err != nil {
		returnResp(conn, "400", "Not Found", "text/plain", []byte(err.Error()))
		return
	}
	defer target.Close()

	targetStat, err := target.Stat()
	checkErr(err)
	// if directory, we shows a list of file/folder
	// if file, we return content
	if targetStat.IsDir() {
		dirs, err := target.Readdir(0)
		checkErr(err)
		if len(dirs) == 0 {
			returnResp(conn, "400", "Not Found", "text/plain", []byte("Empty folder"))
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
		content, err := ioutil.ReadAll(target)
		checkErr(err)
		contentType := getContentType(targetStat.Name())
		returnResp(conn, "200", "OK", contentType, content)
	}
}

// serverFile is function to start server and loop getting any new connection
func serveFile(listener net.Listener) {
	log.Println("Start File Server...")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		defer conn.Close()
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
	checkErr(err)
	// Start file server
	go serveFile(listener)
	// Handle SIGINT and SIGTERM.
	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
	// Spawn close server routines which wait until all requests are done
	closeServer(listener)
}
