package dynamic

import (
	"net"
	"time"
)

type AddressInfo struct {
	validUntil time.Time
	addr       *net.IP
}

type BackendInfo struct {
	addresses        map[string]*AddressInfo
	nextCleanupAfter *time.Time
}

func (b *BackendInfo) calcNextCleanup() {
	var minValidity *time.Time = nil
	for _, info := range b.addresses {
		if minValidity == nil || minValidity.After(info.validUntil) {
			minValidity = &info.validUntil
		}
	}
	b.nextCleanupAfter = minValidity
}
