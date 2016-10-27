package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/tagflag"
	"github.com/naoina/toml"
)

var (
	flags struct {
		C string
	}
)

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

func handleFrontendServeError(err error, frontendName string) {
	log.Fatalf("error serving frontend %q: %s", frontendName, err)
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	tagflag.Parse(&flags)
	config := loadConfig()
	proxy := Proxy{Config: config}
	proxy.createClients()
	// log.Printf("%#v", config)
	for name, fe := range config.Frontends {
		srv := http.Server{
			Addr:    fe.Addr,
			Handler: handler{fe, &proxy},
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
