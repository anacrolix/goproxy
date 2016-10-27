package main

import (
	"crypto/tls"
	"log"
	"net/http"
)

type Proxy struct {
	Config  Config
	Clients map[string]*http.Client
}

func NewProxy(cfg Config) *Proxy {
	p := &Proxy{
		Config: cfg,
	}
	p.createClients()
	return p
}

func (p *Proxy) addClient(name string, c Client) {
	hc := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			log.Print("not following redirect")
			return http.ErrUseLastResponse
		},
	}
	t := http.Transport{}
	tcc := tls.Config{}
	tcc.InsecureSkipVerify = c.SkipVerify
	t.TLSClientConfig = &tcc
	hc.Transport = &t
	p.Clients[name] = hc
}

func (p *Proxy) createClients() {
	cfg := p.Config
	p.Clients = make(map[string]*http.Client)
	for name, c := range cfg.Clients {
		p.addClient(name, c)
	}
	if _, ok := p.Clients[""]; !ok {
		p.addClient("", Client{})
	}
}
