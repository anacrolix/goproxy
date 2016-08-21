# goproxy
HAProxy but not arcane

# running

`godo github.com/anacrolix/goproxy -c=cfg`

https://github.com/anacrolix/godo

# configuration

certs are in pem files in ./certs. default.pem is the certificate used if there's no match with SNI.

```
default_backend = "webtorrent"

[frontends.http]
addr = ":8080"

[frontends.http.redirect]
scheme = "https"
port = 8443

[frontends.https]
addr = ":8443"
tls = true

[backends.webtorrent]
addr = "localhost:8081"

[backends.chromecast]
addr = "localhost:8081"
hosts = ["cast.anacrolix.link"]
```

