// Package etcd provides an etcd service registry
package etcd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/5idu/aurora/registry"
	hash "github.com/mitchellh/hashstructure"
	"github.com/rs/zerolog/log"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	Prefix = "/aurora/registry/"
)

type etcdRegistry struct {
	client  *clientv3.Client
	options registry.Options

	sync.RWMutex
	register map[string]uint64
	leases   map[string]clientv3.LeaseID
}

func NewRegistry(opts ...registry.Option) (registry.Registry, error) {
	e := &etcdRegistry{
		options:  registry.Options{},
		register: make(map[string]uint64),
		leases:   make(map[string]clientv3.LeaseID),
	}

	if err := configure(e, opts...); err != nil {
		return nil, err
	}
	return e, nil
}

func configure(e *etcdRegistry, opts ...registry.Option) error {
	config := clientv3.Config{}

	for _, o := range opts {
		o(&e.options)
	}

	if e.options.Timeout == 0 {
		e.options.Timeout = 5 * time.Second
	}
	config.DialTimeout = e.options.Timeout

	if e.options.Secure || e.options.TLSConfig != nil {
		tlsConfig := e.options.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
		}

		config.TLS = tlsConfig
	}

	if e.options.Context != nil {
		u, ok := e.options.Context.Value(authKey{}).(*authCreds)
		if ok {
			config.Username = u.Username
			config.Password = u.Password
		}
	}

	var cAddrs []string

	for _, address := range e.options.Addrs {
		if len(address) == 0 {
			continue
		}
		addr, port, err := net.SplitHostPort(address)
		if ae, ok := err.(*net.AddrError); ok && ae.Err == "missing port in address" {
			port = "2379"
			addr = address
			cAddrs = append(cAddrs, net.JoinHostPort(addr, port))
		} else if err == nil {
			cAddrs = append(cAddrs, net.JoinHostPort(addr, port))
		}
	}

	// if we got addrs then we'll update
	if len(cAddrs) > 0 {
		config.Endpoints = cAddrs
	} else {
		config.Endpoints = []string{"http://127.0.0.1:2379"}
	}

	cli, err := clientv3.New(config)
	if err != nil {
		return err
	}
	e.client = cli
	return nil
}

func encode(s *registry.Service) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func decode(ds []byte) *registry.Service {
	var s *registry.Service
	json.Unmarshal(ds, &s)
	return s
}

func (e *etcdRegistry) servicePath(s string) string {
	return path.Join(Prefix, strings.Replace(s, "/", "-", -1))
}

func (e *etcdRegistry) Init(opts ...registry.Option) error {
	return configure(e, opts...)
}

func (e *etcdRegistry) Options() registry.Options {
	e.RLock()
	opts := e.options
	e.RUnlock()
	return opts
}

func (e *etcdRegistry) registerNode(s *registry.Service, opts ...registry.RegisterOption) error {
	// check existing lease cache
	e.RLock()
	leaseID, ok := e.leases[s.Name]
	e.RUnlock()

	if !ok {
		// missing lease, check if the key exists
		ctx, cancel := context.WithTimeout(context.Background(), e.options.Timeout)
		defer cancel()

		// look for the existing key
		rsp, err := e.client.Get(ctx, e.servicePath(s.Name), clientv3.WithSerializable())
		if err != nil {
			return err
		}

		// get the existing lease
		for _, kv := range rsp.Kvs {
			if kv.Lease > 0 {
				leaseID = clientv3.LeaseID(kv.Lease)

				// decode the existing node
				srv := decode(kv.Value)
				if srv == nil {
					continue
				}

				// create hash of service; uint64
				h, err := hash.Hash(srv.Name, nil)
				if err != nil {
					continue
				}

				// save the info
				e.Lock()
				e.leases[s.Name] = leaseID
				e.register[s.Name] = h
				e.Unlock()

				break
			}
		}
	}

	var leaseNotFound bool

	// renew the lease if it exists
	if leaseID > 0 {
		log.Info().Msgf("renewing existing lease for %s %d", s.Name, leaseID)
		if _, err := e.client.KeepAliveOnce(context.TODO(), leaseID); err != nil {
			if err != rpctypes.ErrLeaseNotFound {
				return err
			}

			log.Info().Msgf("lease not found for %s %d", s.Name, leaseID)
			// lease not found do register
			leaseNotFound = true
		}
	}

	// create hash of service; uint64
	h, err := hash.Hash(s.Name, nil)
	if err != nil {
		return err
	}

	// get existing hash for the service node
	e.RLock()
	v, ok := e.register[s.Name]
	e.RUnlock()

	// the service is unchanged, skip registering
	if ok && v == h && !leaseNotFound {
		log.Info().Msgf("service %s unchanged skipping registration", s.Name)
		return nil
	}

	service := &registry.Service{
		Name:     s.Name,
		Host:     s.Host,
		Port:     s.Port,
		Metadata: s.Metadata,
	}

	var options registry.RegisterOptions
	for _, o := range opts {
		o(&options)
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.options.Timeout)
	defer cancel()

	var lgr *clientv3.LeaseGrantResponse
	if options.TTL.Seconds() > 0 {
		// get a lease used to expire keys since we have a ttl
		lgr, err = e.client.Grant(ctx, int64(options.TTL.Seconds()))
		if err != nil {
			return err
		}
		log.Info().Msgf("registering %s with leaseID %v and ttl %v", service.Name, lgr.ID, options.TTL)
	} else {
		log.Info().Msgf("registering %s with no ttl", service.Name)
	}
	// create an entry for the service
	if lgr != nil {
		_, err = e.client.Put(ctx, e.servicePath(service.Name), encode(service), clientv3.WithLease(lgr.ID))
	} else {
		_, err = e.client.Put(ctx, e.servicePath(service.Name), encode(service))
	}
	if err != nil {
		return err
	}

	e.Lock()
	// save our hash of the service
	e.register[s.Name] = h
	// save our leaseID of the service
	if lgr != nil {
		e.leases[s.Name] = lgr.ID
	}
	e.Unlock()

	return nil
}

func (e *etcdRegistry) Deregister(s *registry.Service, opts ...registry.DeregisterOption) error {
	e.Lock()
	// delete our hash of the service
	delete(e.register, s.Name)
	// delete our lease of the service
	delete(e.leases, s.Name)
	e.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), e.options.Timeout)
	defer cancel()

	log.Info().Msgf("deregistering %s", s.Name)
	_, err := e.client.Delete(ctx, e.servicePath(s.Name))
	if err != nil {
		return err
	}

	return nil
}

func (e *etcdRegistry) Register(s *registry.Service, opts ...registry.RegisterOption) error {
	if err := e.registerNode(s, opts...); err != nil {
		return err
	}
	return nil
}

func (e *etcdRegistry) GetService(name string, opts ...registry.GetOption) (*registry.Service, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e.options.Timeout)
	defer cancel()

	rsp, err := e.client.Get(ctx, e.servicePath(name), clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		return nil, err
	}

	if len(rsp.Kvs) == 0 {
		return nil, registry.ErrNotFound
	}

	for _, n := range rsp.Kvs {
		if sn := decode(n.Value); sn != nil {
			return sn, nil
		}
	}

	return nil, registry.ErrNotFound
}

func (e *etcdRegistry) ListServices(opts ...registry.ListOption) ([]*registry.Service, error) {
	ctx, cancel := context.WithTimeout(context.Background(), e.options.Timeout)
	defer cancel()

	rsp, err := e.client.Get(ctx, Prefix, clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		return nil, err
	}

	if len(rsp.Kvs) == 0 {
		return []*registry.Service{}, nil
	}

	services := make([]*registry.Service, 0, len(rsp.Kvs))
	for _, n := range rsp.Kvs {
		sn := decode(n.Value)
		if sn == nil {
			continue
		}
		services = append(services, sn)
	}

	return services, nil
}

func (e *etcdRegistry) Watch(opts ...registry.WatchOption) (registry.Watcher, error) {
	return newEtcdWatcher(e, e.options.Timeout, opts...)
}

func (e *etcdRegistry) String() string {
	return "etcd"
}
