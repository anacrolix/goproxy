package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/httptoo"
)

// Stores state and implements http.Handler for serving a given Frontend.
type handler struct {
	Frontend
	*Proxy
}

func (h handler) backendForRequest(r *http.Request) *Backend {
	for _, be := range h.Config.Backends {
		for _, h := range be.Hosts {
			if h == r.Host {
				return &be
			}
		}
	}
	be, ok := h.Config.Backends[h.Config.DefaultBackend]
	if !ok {
		return nil
	}
	return &be
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("got request for %q", httptoo.RequestedURL(r).String())
	u := httptoo.CopyURL(r.URL)
	be := h.backendForRequest(r)
	if rd := be.Redirect; rd != nil {
		if rd.Scheme != "" {
			u.Scheme = rd.Scheme
		}
		if rd.Port != 0 {
			hmp := missinggo.SplitHostMaybePort(r.Host)
			if hmp.Err != nil {
				panic(hmp.Err)
			}
			hmp.Port = rd.Port
			hmp.NoPort = false
			u.Host = hmp.String()
		} else {
			u.Host = r.Host
		}
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	if be == nil {
		log.Printf("%q: no backend", httptoo.RequestedURL(r))
		http.Error(w, "no backend", http.StatusServiceUnavailable)
		return
	}
	u.Scheme = "http"
	if be.Scheme != "" {
		u.Scheme = be.Scheme
	}
	u.Host = be.Addr
	cl := h.Clients[be.Client]
	if cl == nil {
		panic(fmt.Sprintf("no client %q", be.Client))
	}
	err := httptoo.ReverseProxy(w, r, u.String(), cl)
	if err != nil {
		log.Printf("error proxying a request: %s", err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
	}
}
