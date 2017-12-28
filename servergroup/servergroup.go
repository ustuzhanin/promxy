package servergroup

import (
	"context"
	"net/url"
	"sync/atomic"

	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/relabel"
)

// TODO: pass in parent context
func New() *ServerGroup {
	ctx, ctxCancel := context.WithCancel(context.Background())
	// Create the targetSet (which will maintain all of the updating etc. in the background)
	sg := &ServerGroup{
		ctx:       ctx,
		ctxCancel: ctxCancel,
		Ready:     make(chan struct{}),
	}
	sg.targetSet = discovery.NewTargetSet(sg)
	// Background the updating
	go sg.targetSet.Run(sg.ctx)

	return sg

}

type ServerGroup struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	loaded bool
	Ready  chan struct{}

	Cfg       *Config
	targetSet *discovery.TargetSet

	OriginalURLs []string

	urls atomic.Value
}

func (s *ServerGroup) Cancel() {
	s.ctxCancel()
}

func (s *ServerGroup) Sync(tgs []*config.TargetGroup) {
	targets := make([]string, 0)
	for _, tg := range tgs {
		for _, target := range tg.Targets {

			target = relabel.Process(target, s.Cfg.RelabelConfigs...)
			// Check if the target was dropped.
			if target == nil {
				continue
			}

			u := &url.URL{
				Scheme: string(s.Cfg.GetScheme()),
				Host:   string(target[model.AddressLabel]),
			}
			targets = append(targets, u.String())
		}
	}
	s.urls.Store(targets)

	if !s.loaded {
		s.loaded = true
		close(s.Ready)
	}
}

func (s *ServerGroup) ApplyConfig(cfg *Config) error {
	s.Cfg = cfg
	// TODO: make a better wrapper for the log? They made their own... :/
	providerMap := discovery.ProvidersFromConfig(cfg.Hosts, log.Base())
	s.targetSet.UpdateProviders(providerMap)
	return nil
}

func (s *ServerGroup) Targets() []string {
	tmp := s.urls.Load()
	if ret, ok := tmp.([]string); ok {
		return ret
	} else {
		return nil
	}
}