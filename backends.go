package dynamic

import (
	"errors"
	"net"
	"time"
)

func (d *Dynamic) registerBackend(backend string, addr string) error {
	// parse and normalize ip
	ip := net.ParseIP(addr)
	if ip == nil {
		return errors.New("Couldn't parse ip address")
	}
	addr = ip.String()

	backendInfo, ok := d.backends[backend]
	if !ok {
		backendInfo = &BackendInfo{
			addresses: make(map[string]*AddressInfo),
		}
		d.backends[backend] = backendInfo
	}

	addrInfo, ok := backendInfo.addresses[addr]
	if !ok {
		log.Info("Registering " + addr + " for " + backend)
		addrInfo = &AddressInfo{
			addr: &ip,
		}
		backendInfo.addresses[addr] = addrInfo
		d.lastAddrChanged = time.Now()
	}

	addrInfo.validUntil = time.Now().Add(d.keepBackendDuration)
	if d.nextCleanupAfter == nil {
		d.nextCleanupAfter = &addrInfo.validUntil
	}

	backendInfo.calcNextCleanup()
	d.calcNextClanupTime()

	return nil
}

func (d *Dynamic) calcNextClanupTime() {
	var minValidity *time.Time = nil
	for _, info := range d.backends {
		if minValidity == nil || minValidity.After(*info.nextCleanupAfter) {
			minValidity = info.nextCleanupAfter
		}
	}

	d.nextCleanupAfter = minValidity
}

/// Remove expired backends form memory
func (d *Dynamic) cleanup() {
	now := time.Now()
	for backendName, info := range d.backends {
		if now.After(*info.nextCleanupAfter) {
			backendNeedRefresh := false
			for addr, addrInfo := range info.addresses {
				if now.After(addrInfo.validUntil) {
					delete(info.addresses, addr)
					d.lastAddrChanged = time.Now()
					backendNeedRefresh = true
					log.Info("Removing " + addr + " from " + backendName)
				}
			}
			// there is no addresses for this backend, we can remove it
			if len(info.addresses) == 0 {
				delete(d.backends, backendName)
			} else if backendNeedRefresh {
				info.calcNextCleanup()
			}
		}
	}
	d.calcNextClanupTime()
}

/// Run cleanup only when enough time passed
func (d *Dynamic) maybeCleanup() {

	// We do not have
	if d.nextCleanupAfter == nil {
		return
	}

	if time.Now().After(*d.nextCleanupAfter) {
		go d.cleanup()
	}
}
