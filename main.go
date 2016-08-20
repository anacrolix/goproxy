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

var flags struct {
	C string
}

type Frontend struct {
	Addr string
	TLS  bool
}

type Backend struct {
	Hosts []string
	Addr  string
}

var config struct {
	DefaultBackend string
	Frontends      map[string]Frontend
	Backends       map[string]Backend
}

func loadConfig() {
	f, err := os.Open(flags.C)
	if err != nil {
		log.Fatalf("error opening config file: %s", err)
	}
	defer f.Close()
	err = toml.NewDecoder(f).Decode(&config)
	if err != nil {
		log.Fatalf("error decoding config: %s", err)
	}
}

func backendForRequest(r *http.Request) Backend {
	for _, be := range config.Backends {
		for _, h := range be.Hosts {
			if h == r.Host {
				return be
			}
		}
	}
	return config.Backends[config.DefaultBackend]
}

type handler struct {
	Frontend Frontend
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	be := backendForRequest(r)
	u := httptoo.CopyURL(r.URL)
	u.Scheme = "http"
	u.Host = be.Addr
	err := httptoo.ReverseProxy(w, r, u.String(), &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
	if err != nil {
		log.Printf("error proxying a request: %s", err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
	}
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	tagflag.Parse(&flags)
	loadConfig()
	// log.Printf("%#v", config)
	for name, fe := range config.Frontends {
		srv := http.Server{
			Addr:    fe.Addr,
			Handler: handler{fe},
		}
		if fe.TLS {
			srv.TLSConfig = &tls.Config{}
			var err error
			srv.TLSConfig.Certificates, err = missinggo.LoadCertificateDir("certs")
			if err != nil {
				log.Fatalf("error loading certificates from %q: %s", "certs", err)
			}
			go func() {
				err := srv.ListenAndServeTLS("", "")
				log.Fatalf("error serving frontend %q: %s", name, err)
			}()
		}
	}
	select {}
}
