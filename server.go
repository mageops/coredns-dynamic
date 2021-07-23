package dynamic

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/coredns/coredns/plugin/pkg/reuseport"
)

func parseIp(s string) (string, error) {
	ip, _, err := net.SplitHostPort(s)
	if err == nil {
		return ip, nil
	}

	ip2 := net.ParseIP(s)
	if ip2 == nil {
		return "", errors.New("invalid IP")
	}

	return ip2.String(), nil
}

func (d *Dynamic) handleRegister(rw http.ResponseWriter, req *http.Request) {
	if req.Header.Get("secret") == "" {
		log.Debug("Missing secret")
		rw.WriteHeader(401)
		return
	}
	if req.Header.Get("secret") != d.secret {
		log.Debug("Invalid secret received")
		rw.WriteHeader(403)
		return
	}

	backendName := req.Header.Get("backend")
	if backendName == "" {
		log.Debug("Missing backend name")
		rw.WriteHeader(400)
		return
	}

	remoteAddr, err := parseIp(req.RemoteAddr)
	if err != nil {
		log.Error(err)
		rw.WriteHeader(500)
		return
	}

	d.registerBackend(backendName, remoteAddr)
}

func (d *Dynamic) OnStartup() error {
	ln, err := reuseport.Listen("tcp", d.addr)
	if err != nil {
		log.Errorf("Failed to start dynamic backends server: %s", err)
		return err
	}

	d.ln = ln
	d.lnSetup = true

	d.mux = http.NewServeMux()
	d.mux.Handle("/register", &handleWrapper{handler: d.handleRegister})

	server := &http.Server{Handler: d.mux}
	d.srv = server

	go func() {
		server.Serve(ln)
	}()

	return nil
}

// OnRestart stops the listener on reload.
func (d *Dynamic) OnRestart() error {
	if !d.lnSetup {
		return nil
	}
	return d.stopServer()
}

func (d *Dynamic) stopServer() error {
	if !d.lnSetup {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err := d.srv.Shutdown(ctx); err != nil {
		log.Infof("Failed to stop prometheus http server: %s", err)
		return err
	}
	d.lnSetup = false
	d.ln.Close()
	return nil
}

// OnFinalShutdown tears down the metrics listener on shutdown and restart.
func (d *Dynamic) OnFinalShutdown() error {
	return d.stopServer()
}

type handleWrapper struct {
	handler func(rw http.ResponseWriter, req *http.Request)
}

func (h *handleWrapper) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.handler(rw, req)
}
