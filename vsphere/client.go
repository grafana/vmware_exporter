package vsphere

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

type clientFactory struct {
	client     *client
	mux        sync.Mutex
	vSphereURL *url.URL
	cfg        *vSphereConfig
}

// client represents a connection to vSphere and is backed by a govmomi connection
type client struct {
	Client    *govmomi.Client
	Views     *view.Manager
	Root      *view.ContainerView
	Perf      *performance.Manager
	Valid     bool
	Timeout   time.Duration
	closeGate sync.Once
	logger    log.Logger
}

// newClientFactory creates a new clientFactory and prepares it for use.
func newClientFactory(vSphereURL *url.URL, cfg *vSphereConfig) *clientFactory {
	return &clientFactory{
		client:     nil,
		cfg:        cfg,
		vSphereURL: vSphereURL,
	}
}

// GetClient returns a client.
func (cf *clientFactory) GetClient(ctx context.Context) (*client, error) {
	cf.mux.Lock()
	defer cf.mux.Unlock()
	retrying := false
	for {
		if cf.client == nil {
			var err error
			if cf.client, err = newClient(ctx, cf.vSphereURL, cf.cfg); err != nil {
				return nil, err
			}
		}

		// Execute a dummy call against the server to make sure the client is
		// still functional. If not, try to log back in. If that doesn't work,
		// we give up.
		ctx1, cancel1 := context.WithTimeout(ctx, cf.cfg.Timeout)
		defer cancel1()
		if _, err := methods.GetCurrentTime(ctx1, cf.client.Client); err != nil {
			//cf.cfg.Log.Info("client session seems to have time out. Reauthenticating!")
			ctx2, cancel2 := context.WithTimeout(ctx, cf.cfg.Timeout)
			defer cancel2()
			if err := cf.client.Client.SessionManager.Login(ctx2, url.UserPassword(cf.cfg.Username, cf.cfg.Password)); err != nil {
				if !retrying {
					// The client went stale. Probably because someone rebooted vCenter. Clear it to
					// force us to create a fresh one. We only get one chance at this. If we fail a second time
					// we will simply skip this collection round and hope things have stabilized for the next one.
					retrying = true
					cf.client = nil
					continue
				}
				return nil, fmt.Errorf("renewing authentication failed: %s", err.Error())
			}
		}

		return cf.client, nil
	}
}

// newClient creates a new vSphere client based on the url and setting passed as parameters.
// TODO: tls config
func newClient(ctx context.Context, vSphereURL *url.URL, cfg *vSphereConfig) (*client, error) {
	if cfg.Username != "" {
		vSphereURL.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	soapClient := soap.NewClient(vSphereURL, true)

	ctx1, cancel1 := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel1()
	vimClient, err := vim25.NewClient(ctx1, soapClient)
	if err != nil {
		return nil, err
	}
	sm := session.NewManager(vimClient)

	// Create the govmomi client.
	c := &govmomi.Client{
		Client:         vimClient,
		SessionManager: sm,
	}

	// Only login if the URL contains user information.
	if vSphereURL.User != nil {
		if err := c.Login(ctx, vSphereURL.User); err != nil {
			return nil, err
		}
	}

	c.Timeout = cfg.Timeout
	m := view.NewManager(c.Client)

	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{}, true)
	if err != nil {
		return nil, err
	}

	p := performance.NewManager(c.Client)

	client := &client{
		Client:  c,
		Views:   m,
		Root:    v,
		Perf:    p,
		Valid:   true,
		Timeout: cfg.Timeout,
	}
	return client, nil
}

// counterInfoByKey wraps performance.CounterInfoByKey to give it proper timeouts
func (c *client) counterInfoByKey(ctx context.Context) (map[int32]*types.PerfCounterInfo, error) {
	ctx1, cancel1 := context.WithTimeout(ctx, c.Timeout)
	defer cancel1()
	return c.Perf.CounterInfoByKey(ctx1)
}

// CounterInfoByName wraps performance.CounterInfoByName to give it proper timeouts
func (c *client) counterInfoByName(ctx context.Context) (map[string]*types.PerfCounterInfo, error) {
	ctx1, cancel1 := context.WithTimeout(ctx, c.Timeout)
	defer cancel1()
	return c.Perf.CounterInfoByName(ctx1)
}
