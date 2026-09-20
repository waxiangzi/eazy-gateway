module github.com/eazy-gateway/eazy-gateway

go 1.26.3

require (
	golang.org/x/crypto v0.52.0
	go.etcd.io/bbolt v0.0.0-00010101000000-000000000000
)

replace go.etcd.io/bbolt => ./internal/bbolt_shim
