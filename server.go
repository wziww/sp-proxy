// +build ignore

package main

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/judwhite/go-svc/svc"
)

type program struct {
}

var cc *websocket.Conn
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	prg := &program{}
	if err := svc.Run(prg, syscall.SIGINT, syscall.SIGTERM); err != nil {
		fmt.Println(err)
	}
}
func (p *program) Init(e svc.Environment) error {
	return nil
}

var (
	lock sync.Mutex
)
var chanpool map[string]chan (*socketReq)
var sppool map[string]chan ([]byte)
var (
	chanpoolLock sync.RWMutex
	sppoolLock   sync.RWMutex
)

func init() {
	chanpool = make(map[string]chan (*socketReq))
	sppool = make(map[string]chan []byte)
}
func (p *program) Start() error {
	go func() {
		http.HandleFunc(C.Server.Path, ws)
		http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(res, req.Body, 200<<20)
			proxyAll(res, req)
		})
		log.Fatal(http.ListenAndServe("0.0.0.0:"+C.Server.Port, nil))
	}()
	return nil
}
func proxyAll(res http.ResponseWriter, req *http.Request) {
	proxyReq := &socketReq{}
	key := fmt.Sprintf("%x", md5.Sum([]byte(uuid.New().String())))
	chanpoolLock.Lock()
	chanpool[key] = make(chan *socketReq)
	chanpoolLock.Unlock()
	proxyReq.Method = req.Method
	proxyReq.URL = req.RequestURI
	proxyReq.Key = key
	proxyReq.Header = make(map[string]string, 0)
	if len(req.Header) > 0 {
		for k, v := range req.Header {
			proxyReq.Header[k] = v[0]
		}
	}
	br, _ := ioutil.ReadAll(req.Body)
	proxyReq.Body = br
	defer func() {
		chanpoolLock.Lock()
		delete(chanpool, key)
		chanpoolLock.Unlock()
	}()
	lock.Lock()
	if cc != nil {
		buf := bytes.NewBuffer(nil)
		enc := gob.NewEncoder(buf)
		enc.Encode(proxyReq)
		e := cc.WriteMessage(websocket.BinaryMessage, buf.Bytes())
		fmt.Println(e)
	}
	lock.Unlock()
	select {
	case <-time.After(time.Second * 60):
		res.WriteHeader(504)
		return
	case t := <-chanpool[key]:
		for k, v := range t.Header {
			res.Header().Set(k, v)
		}
		res.WriteHeader(t.Code)
		res.Write(t.Body)
	}
}
func (p *program) Stop() error {
	fmt.Println("exit")
	return nil
}

func ws(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	cc = c
	defer c.Close()
	for {
		_, resData, err := c.ReadMessage()
		if err != nil {
			fmt.Println(err)
			e := err.(*websocket.CloseError)
			if e.Code == websocket.CloseGoingAway || e.Code == websocket.CloseAbnormalClosure { // conn close
				cc = nil
			}
			break
		}
		reqData := &socketReq{}
		buf := bytes.NewBuffer(resData)
		dec := gob.NewDecoder(buf)
		dec.Decode(reqData)
		chanpoolLock.RLock()
		v := chanpool[reqData.Key]
		chanpoolLock.RUnlock()
		if v != nil {
			chanpoolLock.Lock()
			chanpool[reqData.Key] <- reqData
			chanpoolLock.Unlock()
		}

	}
}
