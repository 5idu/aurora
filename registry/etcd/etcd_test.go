package etcd

import (
	"os"
	"testing"
	"time"

	"github.com/5idu/aurora/registry"
)

var (
	testData = []*registry.Service{
		{
			Name: "foo",
			Host: "localhost",
			Port: 8080,
			Metadata: map[string]interface{}{
				"foo": "bar",
			},
		},
		{
			Name: "bar",
			Host: "localhost",
			Port: 8081,
			Metadata: map[string]interface{}{
				"bar": "baz",
			},
		},
	}
)

func TestEtcdRegistry(t *testing.T) {
	m, err := NewRegistry(registry.Addrs("http://127.0.0.1:2379"))
	if err != nil {
		t.Fatal(err)
	}

	// register data
	for _, v := range testData {
		if err := m.Register(v); err != nil {
			t.Errorf("Unexpected register error: %v", err)
		}
		// after the service has been registered we should be able to query it
		service, err := m.GetService(v.Name)
		if err != nil {
			t.Errorf("Unexpected error getting service %s: %v", service.Name, err)
		}
		if service.Name != v.Name {
			t.Errorf("Expected %s service, got %s", v.Name, service.Name)
		}
	}

	services, err := m.ListServices()
	if err != nil {
		t.Errorf("Unexpected error when listing services: %v", err)
	}

	if len(services) != len(testData) {
		t.Errorf("Expected total service count: %d, got: %d", len(testData), len(services))
	}

	// deregister
	for _, v := range testData {
		if err := m.Deregister(v); err != nil {
			t.Errorf("Unexpected deregister error: %v", err)
		}
	}

	// after all the service nodes have been deregistered we should not get any results
	for _, v := range testData {
		_, err := m.GetService(v.Name)
		if err != registry.ErrNotFound {
			t.Errorf("Expected error: %v, got: %v", registry.ErrNotFound, err)
		}
	}
}

func TestEtcdRegistryTTL(t *testing.T) {
	m, err := NewRegistry(registry.Addrs("http://127.0.0.1:2379"))
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range testData {
		if err := m.Register(v, registry.RegisterTTL(2*time.Second)); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(3 * time.Second)

	for _, v := range testData {
		_, err := m.GetService(v.Name)
		if err != nil && err != registry.ErrNotFound {
			t.Fatal(err)
		}
	}
}

func TestEtcdRegistryTTLConcurrent(t *testing.T) {
	concurrency := 1000
	waitTime := time.Second * 2
	m, err := NewRegistry(registry.Addrs("http://127.0.0.1:2379"))
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range testData {
		if err := m.Register(v, registry.RegisterTTL(waitTime/2)); err != nil {
			t.Fatal(err)
		}
	}

	if len(os.Getenv("IN_TRAVIS_CI")) == 0 {
		t.Logf("test will wait %v, then check TTL timeouts", waitTime)
	}

	errChan := make(chan error, concurrency)
	syncChan := make(chan struct{})

	for i := 0; i < concurrency; i++ {
		go func() {
			<-syncChan
			for _, v := range testData {
				_, err := m.GetService(v.Name)
				if err != nil && err != registry.ErrNotFound {
					errChan <- err
					return
				}

			}

			errChan <- nil
		}()
	}

	time.Sleep(waitTime)
	close(syncChan)

	for i := 0; i < concurrency; i++ {
		if err := <-errChan; err != nil {
			t.Fatal(err)
		}
	}
}
