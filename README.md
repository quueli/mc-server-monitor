# mc-server-monitor

![ci](https://github.com/quueli/mc-server-monitor/actions/workflows/ci.yml/badge.svg)

pings minecraft servers and keeps a short history of players online / latency, with a small http api on top. go, stdlib only.

the server list ping protocol is done by hand (varint framing, handshake, status request) because the libraries i tried either pulled half the internet in or died on odd motd json. see internal/mcping. srv records are resolved so `play.example.com` works without a port.

polling runs in a worker pool, every server gets a goroutine slot and a panic in one ping cant take the loop down. results are cached with a ttl so the api never triggers a ping by itself.

## run

    MONITOR_SERVERS=play.hypixel.net,mc.example.com go run ./cmd/server
    curl localhost:8080/servers

    go test ./...

the token bucket in internal/httpapi is per ip, 60 req/min by default. storage is in-memory unless you point it at sqlite, migrations are in migrations/.
