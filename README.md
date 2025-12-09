# mc-server-monitor

pings minecraft servers and keeps a short history of players online / latency, with a small http api on top. go, stdlib only.

    MONITOR_SERVERS=play.hypixel.net,mc.example.com go run ./cmd/server
    curl localhost:8080/servers
