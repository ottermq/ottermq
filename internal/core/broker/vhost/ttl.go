package vhost

type TTLManager interface {
	// Define methods for TTL management if needed
}
type DefaultTTLManager struct {
	vh *VHost
}

type NoOpTTLManager struct{}
