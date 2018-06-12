package rpc

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/loomnetwork/loomchain/log"
)

type CombinedProxy struct {
	httputil.ReverseProxy
	Dial func(network, addr string) (net.Conn, error)

	// TLSClientConfig specifies the TLS configuration to use for 'wss'.
	// If nil, the default configuration is used.
	TLSClientConfig *tls.Config
}

func RunRPCProxyServer(listenPort, rpcPort int32, queryPort int32) error {
	proxyServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", listenPort),
		Handler: rpcProxy(rpcPort, queryPort),
	}
	return proxyServer.ListenAndServe()
}

func rmEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}

func rpcProxy(rpcPort int32, queryPort int32) http.HandlerFunc {
	director := func(req *http.Request) {
		if strings.HasPrefix(req.RequestURI, "/rpc") {
			req.URL.Host = fmt.Sprintf("127.0.0.1:%d", rpcPort)
			req.URL.Scheme = "http"
			req.RequestURI = ""
		} else if strings.HasPrefix(req.RequestURI, "/queryws") {
			req.URL.Host = fmt.Sprintf("127.0.0.1:%d", queryPort)
			req.URL.Path = "/query/queryws"
			req.URL.Scheme = "ws"
			req.RequestURI = ""
		} else if strings.HasPrefix(req.RequestURI, "/query") {
			req.URL.Host = fmt.Sprintf("127.0.0.1:%d", queryPort)
			req.URL.Scheme = "http"
			req.RequestURI = ""
		}
		parts := rmEmpty(strings.SplitN(req.URL.Path, "/", 3))
		if len(parts) == 1 {
			req.URL.Path = "/"
		} else {
			req.URL.Path = "/" + parts[1]
		}
	}

	responseModifier := func(res *http.Response) error {
		res.Header.Add("Access-Control-Allow-Headers", "Content-Type")
		return nil
	}

	revProxy := CombinedProxy{
		httputil.ReverseProxy{Director: director, ModifyResponse: responseModifier},
		nil,
		nil,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/queryws") {
			revProxy.wsServeHTTP(w, r)
		} else {
			revProxy.ServeHTTP(w, r)
		}
	}
}

func (p *CombinedProxy) wsServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !IsWebSocketRequest(r) {
		http.Error(w, "Cannot handle non-WebSocket requests", 500)
		log.Error("Received a request that was not a WebSocket request")
		return
	}

	outreq := new(http.Request)
	// shallow copying
	*outreq = *r
	p.Director(outreq)
	host := outreq.URL.Host

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := outreq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outreq.Header.Set("X-Forwarded-For", clientIP)
	}

	dial := p.Dial
	if dial == nil {
		dial = net.Dial
	}

	if outreq.URL.Scheme == "wss" {
		var tlsConfig *tls.Config
		if p.TLSClientConfig == nil {
			tlsConfig = &tls.Config{}
		} else {
			tlsConfig = p.TLSClientConfig
		}
		dial = func(network, address string) (net.Conn, error) {
			return tls.Dial("tcp", host, tlsConfig)
		}
	}

	d, err := dial("tcp", host)
	if err != nil {
		http.Error(w, "Error forwarding request.", 500)
		log.Error("Error dialing websocket backend %s: %v", outreq.URL, err)
		return
	}
	// All request generated by the http package implement this interface.
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Not a hijacker?", 500)
		return
	}
	// Hijack() tells the http package not to do anything else with the connection.
	// After, it bcomes this functions job to manage it. `nc` is of type *net.Conn.
	nc, _, err := hj.Hijack()
	if err != nil {
		log.Error("Hijack error: %v", err)
		return
	}
	defer nc.Close() // must close the underlying net connection after hijacking
	defer d.Close()

	// write the modified incoming request to the dialed connection
	err = outreq.Write(d)
	if err != nil {
		log.Error("Error copying request to target: %v", err)
		return
	}
	errc := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("Recovered from panic in websocket reverse proxy", "error", r)
			}
		}()
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go cp(d, nc)
	go cp(nc, d)
	select {
	case <-errc:
	case <-r.Context().Done():
	}
}

// IsWebSocketRequest returns a boolean indicating whether the request has the
// headers of a WebSocket handshake request.
func IsWebSocketRequest(r *http.Request) bool {
	contains := func(key, val string) bool {
		vv := strings.Split(r.Header.Get(key), ",")
		for _, v := range vv {
			if val == strings.ToLower(strings.TrimSpace(v)) {
				return true
			}
		}
		return false
	}
	if !contains("Connection", "upgrade") {
		return false
	}
	if !contains("Upgrade", "websocket") {
		return false
	}
	return true
}
