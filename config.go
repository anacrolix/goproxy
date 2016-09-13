package main

type Config struct {
	DefaultBackend string
	Frontends      map[string]Frontend
	Backends       map[string]Backend
	Clients        map[string]Client
}

type Frontend struct {
	Addr string
	TLS  bool
}

type Redirect struct {
	Scheme string
	Port   int
}

type Backend struct {
	Hosts    []string
	Addr     string
	Scheme   string
	Client   string
	Redirect *Redirect
}

type Client struct {
	SkipVerify bool
}
