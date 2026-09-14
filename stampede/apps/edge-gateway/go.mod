module github.com/Rishab-Kumar-R/tf-man/stampede/edge-gateway

go 1.26.5

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

require (
	github.com/Rishab-Kumar-R/tf-man/stampede/internal v0.0.0
	github.com/gorilla/websocket v1.5.3
	github.com/redis/go-redis/v9 v9.22.0
	golang.org/x/sync v0.23.0
)

replace github.com/Rishab-Kumar-R/tf-man/stampede/internal => ../internal
