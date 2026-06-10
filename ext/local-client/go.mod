module github.com/computeflux/tee-provider/ext/local-client

go 1.26.3

require (
	github.com/computeflux/tee-provider v0.0.0
	github.com/libp2p/go-yamux/v5 v5.0.1
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
)

replace github.com/computeflux/tee-provider => ../..
