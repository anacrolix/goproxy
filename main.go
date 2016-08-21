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
	Addr     string
	TLS      bool
	Redirect *Redirect
}

type Redirect struct {
	Scheme string
	Port   int
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

var hc = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u := httptoo.CopyURL(r.URL)
	if rd := h.Frontend.Redirect; rd != nil {
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
	be := backendForRequest(r)
	u.Scheme = "http"
	u.Host = be.Addr
	err := httptoo.ReverseProxy(w, r, u.String(), hc)
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
