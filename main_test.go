package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/http"
	"testing"

	"github.com/anacrolix/missinggo/httptoo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"
)

var testOriginMuxer http.ServeMux

func init() {
	log.SetFlags(log.Flags() | log.Lshortfile)

	testOriginMuxer.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("got request at origin for %q", httptoo.RequestedURL(r))
		websocket.Handler(func(ws *websocket.Conn) {
			var s string
			websocket.JSON.Receive(ws, &s)
			if s != "hello" {
				panic(s)
			}
			websocket.JSON.Send(ws, "greetings")
		}).ServeHTTP(w, r)
	})
	testOriginMuxer.Handle("/redirect", http.RedirectHandler("/simple", 307))
	testOriginMuxer.HandleFunc("/simple", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})
}

// Client that doesn't follow redirects, so we can check the immediate
// response from Proxy.
var rawClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func testOriginHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("got request at origin for %q", httptoo.RequestedURL(r))
	testOriginMuxer.ServeHTTP(w, r)
}

func TestWebsocketNoProxy(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer l.Close()
	go http.Serve(l, http.HandlerFunc(testOriginHandler))
	testClient(t, l.Addr().String())
}

func testWebsocketClient(t *testing.T, addr string) {
	ws, err := websocket.Dial("ws://"+addr, "", "http://some.origin")
	require.NoError(t, err)
	defer ws.Close()
	require.NoError(t, websocket.JSON.Send(ws, "hello"))
	var s string
	require.NoError(t, websocket.JSON.Receive(ws, &s))
	require.EqualValues(t, "greetings", s)
}

func testClientRedirected(t *testing.T, addr string) {
	resp, err := http.Get("http://" + addr + "/redirect")
	require.NoError(t, err)
	resp.Body.Close()
	assert.EqualValues(t, 307, resp.StatusCode)
	t.Log(resp.Status)
}

func testSimpleClient(t *testing.T, hostPath string, expectedResponseCode int, expectedBody string) {
	resp, err := rawClient.Get("http://" + hostPath)
	require.NoError(t, err)
	assert.EqualValues(t, expectedResponseCode, resp.StatusCode)
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	assert.EqualValues(t, expectedBody, buf.String())
}

func testClient(t *testing.T, addr string) {
	testWebsocketClient(t, addr+"/ws")
	testSimpleClient(t, addr+"/simple", 200, "hello")
	testSimpleClient(t, addr+"/redirect", 307, "<a href=\"/simple\">Temporary Redirect</a>.\n\n")
}

func TestWebsocket(t *testing.T) {
	originListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer originListener.Close()
	go http.Serve(originListener, http.HandlerFunc(testOriginHandler))
	proxyListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer proxyListener.Close()
	proxy := NewProxy(Config{
		Backends: map[string]Backend{
			"": Backend{
				Addr: originListener.Addr().String(),
			},
		},
	})
	go http.Serve(proxyListener, handler{
		Proxy: proxy,
	})
	testClient(t, proxyListener.Addr().String())
}

func TestRedirectPassedThrough(t *testing.T) {
	originListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer originListener.Close()
	go http.Serve(originListener, http.HandlerFunc(testOriginHandler))
	proxyListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer proxyListener.Close()
	proxy := NewProxy(Config{
		Backends: map[string]Backend{
			"": Backend{
				Addr:   originListener.Addr().String(),
				Client: "herp",
			},
		},
		Clients: map[string]Client{
			"herp": Client{},
		},
	})
	go http.Serve(proxyListener, handler{
		Proxy: proxy,
	})
	testClient(t, proxyListener.Addr().String())
}
