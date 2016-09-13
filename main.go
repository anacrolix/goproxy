package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/httptoo"
	"github.com/anacrolix/tagflag"
	"github.com/naoina/toml"
)

var (
	flags struct {
		C string
	}
	clients map[string]*http.Client
)

func createClients(cfg Config) {
	for name, c := range cfg.Clients {
		hc := &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		t := http.Transport{}
		tcc := tls.Config{}
		tcc.InsecureSkipVerify = c.SkipVerify
		t.TLSClientConfig = &tcc
		hc.Transport = &t
		clients[name] = hc
	}
}

func loadConfig() (config Config) {
	f, err := os.Open(flags.C)
	if err != nil {
		log.Fatalf("error opening config file: %s", err)
	}
	defer f.Close()
	err = toml.NewDecoder(f).Decode(&config)
	if err != nil {
		log.Fatalf("error decoding config: %s", err)
	}
	return
}

type handler struct {
	Frontend
	Config Config
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
	err := httptoo.ReverseProxy(w, r, u.String(), clients[be.Client])
	if err != nil {
		log.Printf("error proxying a request: %s", err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
	}
}

func handleFrontendServeError(err error, frontendName string) {
	log.Fatalf("error serving frontend %q: %s", frontendName, err)
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	tagflag.Parse(&flags)
	config := loadConfig()
	// log.Printf("%#v", config)
	for name, fe := range config.Frontends {
		srv := http.Server{
			Addr:    fe.Addr,
			Handler: handler{fe, config},
		}
		if fe.TLS {
			srv.TLSConfig = &tls.Config{}
			var err error
			srv.TLSConfig.Certificates, err = missinggo.LoadCertificateDir("certs")
			if err != nil {
				log.Fatalf("error loading certificates from %q: %s", "certs", err)
			}
			srv.TLSConfig.BuildNameToCertificate()
			go func() {
				handleFrontendServeError(srv.ListenAndServeTLS("", ""), name)
			}()
		} else {
			go func() {
				handleFrontendServeError(srv.ListenAndServe(), name)
			}()
		}
	}
	select {}
}
