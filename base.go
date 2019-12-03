package main

type socketReq struct {
	Method string
	URL    string
	Header map[string]string
	Code   int
	Key    string
	Body   []byte
}
