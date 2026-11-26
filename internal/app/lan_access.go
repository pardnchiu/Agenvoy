package app

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
)

var (
	lanMu        sync.Mutex
	lanListeners []net.Listener
	lanNets      []*net.IPNet
)

func SyncLANListeners(server *http.Server, port string, enable bool) []*net.IPNet {
	lanMu.Lock()
	defer lanMu.Unlock()

	if !enable {
		for _, listener := range lanListeners {
			_ = listener.Close()
		}
		if len(lanListeners) > 0 {
			slog.Info("LAN access disabled")
		}
		lanListeners = nil
		lanNets = nil
		return nil
	}
	if len(lanListeners) > 0 {
		return lanNets
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		slog.Warn("net.InterfaceAddrs", slog.String("error", err.Error()))
		return nil
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || !(ipNet.IP.IsPrivate() || (ipNet.IP.To4() != nil && ipNet.IP.IsLinkLocalUnicast())) {
			continue
		}
		listener, err := net.Listen("tcp", net.JoinHostPort(ipNet.IP.String(), port))
		if err != nil {
			slog.Warn("net.Listen", slog.String("addr", ipNet.IP.String()), slog.String("error", err.Error()))
			continue
		}
		lanListeners = append(lanListeners, listener)
		lanNets = append(lanNets, ipNet)
		slog.Warn("LAN access enabled: API reachable from the local network without a password; turn it off when done",
			slog.String("addr", listener.Addr().String()))
		go func(l net.Listener) {
			if err := server.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
				slog.Warn("server.Serve", slog.String("addr", l.Addr().String()), slog.String("error", err.Error()))
			}
		}(listener)
	}
	return lanNets
}
