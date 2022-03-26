package internal

import (
	"encoding/json"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/5idu/aurora/registry"
	"github.com/5idu/aurora/registry/etcd"

	"github.com/5idu/aurora/utils"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// Server is out main struct
type Server struct {
	// global client id
	gcid uint64

	mu               sync.Mutex
	shutdown         atomic.Value
	quitCh           chan struct{}
	shutdownComplete chan struct{}
	done             chan bool

	opts   *Options
	optsMu sync.RWMutex

	startAt time.Time

	httpListener  net.Listener
	pprofListener net.Listener

	tcpListener net.Listener
	clients     map[uint64]*client // tcp connections

	info     ServerInfo
	infoJson []byte
	infoMu   sync.RWMutex

	register registry.Registry
}

type ServerInfo struct {
	Id         string `json:"server_id"`
	Version    string `json:"version"`
	Proto      string `json:"proto"` // protocol version
	GoVersion  string `json:"go_version"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	MaxPayload int    `json:"max_payload"` // maximum payload size, in bytes, that the server will accept from the client.
}

// HTTP endpoints
const (
	HealthPath = "/health"
)

const (
	heartbeatTTL = 1 * time.Minute // every 1 minute we send a heartbeat
	keepaliveTTL = 3 * time.Minute
)

func New(opts *Options) *Server {
	srv := &Server{
		opts: opts,
		info: ServerInfo{
			Id:         utils.GenRandomId(),
			Version:    Version,
			Proto:      Proto,
			GoVersion:  GoVersion,
			Host:       opts.Host,
			Port:       opts.Port,
			MaxPayload: MaxMessageSize,
		},
		startAt:          time.Now(),
		quitCh:           make(chan struct{}),
		shutdownComplete: make(chan struct{}),
		done:             make(chan bool),
		clients:          make(map[uint64]*client),
	}
	srv.shutdown.Store(false)

	infoJson, err := json.Marshal(srv.info)
	if err != nil {
		log.Fatal().Msg(errors.WithMessage(err, "failed to marshal server info").Error())
		return nil
	}
	srv.infoJson = infoJson

	srv.register, err = etcd.NewRegistry(registry.Addrs(opts.EtcdAddr...))
	if err != nil {
		log.Fatal().Msg(errors.WithMessage(err, "failed to init register").Error())
		return nil
	}
	return srv
}

func (srv *Server) Start() error {
	log.Info().Msg("starting aurora server")
	defer log.Info().Msg("server is ready")

	// start pprof
	srv.StartProfiler()

	// start http monitor
	srv.StartMonitoring()

	// register to discovery center
	srv.StartRegister()

	// start tcp listener and accept tcp connections
	srv.AcceptLoop()

	// listen for signals
	srv.HandleSignals()

	return nil
}

// WaitForShutdown will block until the server has been fully shutdown.
func (srv *Server) WaitForShutdown() {
	<-srv.shutdownComplete
}

func (srv *Server) StartRegister() error {
	info := srv.getInfo()
	// register self to discovery center
	rsrv := &registry.Service{
		Name: info.Id,
		Host: info.Host,
		Port: info.Port,
		Metadata: map[string]interface{}{
			"proto":       info.Proto,
			"version":     info.Version,
			"max_payload": info.MaxPayload,
			"go_version":  info.GoVersion,
		},
	}
	err := srv.register.Register(rsrv, registry.RegisterTTL(keepaliveTTL))

	if err != nil {
		return err
	}
	go srv.heartbeat(rsrv)
	return nil
}

func (srv *Server) getOpts() *Options {
	srv.optsMu.RLock()
	opts := srv.opts
	srv.optsMu.RUnlock()
	return opts
}

func (srv *Server) getInfo() ServerInfo {
	srv.infoMu.RLock()
	info := srv.info
	srv.infoMu.RUnlock()
	return info
}

// StartProfiler start pprof http server
func (srv *Server) StartProfiler() {
	opts := srv.getOpts()

	if srv.shutdown.Load().(bool) {
		return
	}

	hp := net.JoinHostPort(opts.Host, strconv.Itoa(opts.ProfPort))
	l, err := net.Listen("tcp", hp)
	if err != nil {
		log.Fatal().Msgf("error starting profiler: %s", err)
		return
	}
	log.Info().Msgf("starting profiler on: %d", l.Addr().(*net.TCPAddr).Port)

	s := &http.Server{
		Addr:           hp,
		Handler:        http.DefaultServeMux,
		MaxHeaderBytes: 1 << 20,
	}
	srv.pprofListener = l

	// Enable blocking profile
	runtime.SetBlockProfileRate(1)

	go func() {
		if err := s.Serve(l); err != nil {
			if !srv.shutdown.Load().(bool) {
				log.Error().Msgf("error starting profiler: %s", err)
			}
		}
		s.Close()
		srv.done <- true
	}()
}

// StartMonitoring start http monitor, like health check etc.
func (srv *Server) StartMonitoring() {
	opts := srv.getOpts()

	hp := net.JoinHostPort(opts.Host, strconv.Itoa(opts.HTTPPort))
	l, err := net.Listen("tcp", hp)
	if err != nil {
		log.Fatal().Msgf("error starting monitor: %s", err)
		return
	}
	log.Info().Msgf("starting monitor on: %d", l.Addr().(*net.TCPAddr).Port)

	mux := http.NewServeMux()
	// Health
	mux.HandleFunc(HealthPath, srv.HandleHealth)

	s := &http.Server{
		Addr:           hp,
		Handler:        mux,
		MaxHeaderBytes: 1 << 20,
	}

	if srv.shutdown.Load().(bool) {
		l.Close()
		return
	}
	srv.httpListener = l

	go func() {
		if err := s.Serve(l); err != nil {
			if !srv.shutdown.Load().(bool) {
				log.Error().Msgf("error starting monitor: %s", err)
			}
		}
		s.Close()
		srv.done <- true
	}()
}

func (srv *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	type HealthStatus struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	// TODO: change resp struct
	resp := HealthStatus{
		Status: "ok",
	}
	b, err := json.Marshal(&resp)
	if err != nil {
		log.Error().Msgf("error marshaling response to /health request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// Signal Handling
func (srv *Server) HandleSignals() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-quit
		log.Info().Msgf("received signal %s, shutting down", sig)
		srv.Shutdown()
		os.Exit(0)
	}()
}

func (srv *Server) Shutdown() {
	if srv == nil {
		return
	}

	// Prevent issues with multiple calls.
	if srv.shutdown.Load().(bool) {
		return
	}
	srv.shutdown.Store(true)

	srv.mu.Lock()
	conns := make(map[uint64]*client)
	// Copy off the clients
	for i, c := range srv.clients {
		conns[i] = c
	}

	// Number of done channel responses we expect.
	doneExpected := 0
	// Kick client AcceptLoop()
	if srv.tcpListener != nil {
		doneExpected++
		srv.tcpListener.Close()
		srv.tcpListener = nil
	}
	// Kick HTTP monitoring if its running
	if srv.httpListener != nil {
		doneExpected++
		srv.httpListener.Close()
		srv.httpListener = nil
	}
	// Kick Profiling if its running
	if srv.pprofListener != nil {
		doneExpected++
		srv.pprofListener.Close()
		srv.pprofListener = nil
	}
	// Kick the server heartbeat and deregister
	doneExpected++

	srv.mu.Unlock()

	// Release go routines that wait on that channel
	close(srv.quitCh)

	// Close client and route connections
	for _, c := range conns {
		c.close()
	}

	// Block until the accept loops exit
	for doneExpected > 0 {
		<-srv.done
		doneExpected--
	}

	log.Info().Msg("server exiting...")
	// Notify that the shutdown is complete
	close(srv.shutdownComplete)
}

func (srv *Server) AcceptLoop() {
	opts := srv.getOpts()

	if srv.shutdown.Load().(bool) {
		return
	}

	hp := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	l, err := net.Listen("tcp", hp)
	if err != nil {
		log.Fatal().Msgf("error starting listener: %s", err)
		return
	}
	log.Info().Msgf("start listening for client connections on: %d", l.Addr().(*net.TCPAddr).Port)

	srv.tcpListener = l

	go srv.acceptConnections(l)
}

func (srv *Server) acceptConnections(l net.Listener) {
	var tempDelay time.Duration // how long to sleep on accept failure

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-srv.quitCh:
				log.Info().Msg("server has been asked to shut down, aborting accept loop")
				break
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Info().Msgf("accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			} else if errors.Is(err, net.ErrClosed) {
				log.Info().Msg("server has been closed, aborting accept loop")
			} else {
				log.Error().Msgf("accept error: %v", err)
			}
			break
		}
		tempDelay = 0
		go srv.createClient(conn)
	}

	log.Info().Msg("accept loop exiting...")
	srv.done <- true
}

func (srv *Server) createClient(conn net.Conn) *client {
	c := &client{
		srv:     srv,
		conn:    conn,
		startAt: time.Now(),
		cid:     atomic.AddUint64(&srv.gcid, 1),
	}

	if srv.shutdown.Load().(bool) {
		conn.Close()
		return c
	}

	log.Info().Msg("client connection created")
	srv.mu.Lock()
	srv.clients[c.cid] = c
	srv.mu.Unlock()

	go c.reedLoop()

	return c
}

func (srv *Server) heartbeat(rsrv *registry.Service) {
	timer := time.NewTicker(heartbeatTTL)
	for {
		select {
		case <-timer.C:
			if err := srv.register.Register(rsrv, registry.RegisterTTL(keepaliveTTL)); err != nil {
				log.Warn().Msg(errors.WithMessage(err, "server keepalive error").Error())
			}
		case <-srv.quitCh:
			log.Info().Msg("server heartbeat job exiting...")
			timer.Stop()
			if err := srv.register.Deregister(rsrv); err != nil {
				log.Warn().Msg(errors.WithMessage(err, "server deregister error").Error())
			}
			srv.done <- true
			return
		}
	}
}
