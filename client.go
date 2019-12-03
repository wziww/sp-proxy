// +build ignore

package main

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: C.Server.Scheme, Host: C.Server.Host, Path: C.Server.Path}
	for {
		connect(u)
	}
}

var (
	lock sync.Mutex
)

func connect(u url.URL) {
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Println(err)
		time.Sleep(3 * time.Second)
		return
	}
	defer c.Close()
	log.Printf("connecting to %s", u.String())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			go func(message []byte) {
				var host string
				reqData := &socketReq{}
				buf := bytes.NewBuffer(message)
				dec := gob.NewDecoder(buf)
				dec.Decode(reqData)
				client := &http.Client{
					Timeout: 50 * time.Second,
				}
				URL := reqData.URL
				for _, v := range C.Routers {
					matched, _ := regexp.MatchString(v.Path, reqData.URL)
					if matched {
						if v.Strip > 0 && v.Strip < len(URL) {
							URL = URL[v.Strip:]
						}
						host = v.Upstream
						if v.CopyStream != "" {
							go func() {
								client := &http.Client{
									Timeout: 60 * time.Second,
								}
								req, err := http.NewRequest(reqData.Method, v.CopyStream+URL, bytes.NewReader(reqData.Body))
								if err != nil {
									fmt.Println(err)
								} else {
									for k, v := range reqData.Header {
										req.Header.Set(k, v)
									}
									_, errres := client.Do(req)
									if errres != nil {
										fmt.Println(errres)
									}
								}
							}()
						}
						break
					}
				}
				req, err := http.NewRequest(reqData.Method, host+URL, bytes.NewReader(reqData.Body))
				if err != nil {
					reqData.Code = 500
					reqData.Body = nil
					lock.Lock()
					c.WriteMessage(websocket.BinaryMessage, *(*[]byte)(unsafe.Pointer(reqData)))
					lock.Unlock()
					return
				}
				for k, v := range reqData.Header {
					req.Header.Set(k, v)
				}
				res, err := client.Do(req)
				if err != nil {
					reqData.Code = 500
					reqData.Body = nil
					lock.Lock()
					c.WriteMessage(websocket.BinaryMessage, *(*[]byte)(unsafe.Pointer(reqData)))
					lock.Unlock()
					return
				}
				reqData.Code = res.StatusCode
				if len(res.Header) > 0 {
					reqData.Header = make(map[string]string, 0)
					for k, v := range res.Header {
						reqData.Header[k] = v[0]
					}
				}
				resD, _ := ioutil.ReadAll(res.Body)
				reqData.Body = resD
				buf = bytes.NewBuffer(nil)
				enc := gob.NewEncoder(buf)
				enc.Encode(reqData)
				lock.Lock()
				c.WriteMessage(websocket.BinaryMessage, buf.Bytes())
				lock.Unlock()
				return
			}(message)
		}
	}()
	select {
	case <-done:
		return
	}
}
