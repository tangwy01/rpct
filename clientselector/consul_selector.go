package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"net/url"
	"strconv"
	"strings"
	"time"

	"../src"
	"github.com/hashicorp/consul/api"
)

// ConsulClientSelector is used to select a rpc server from consul.
//This registry is experimental and has not been test.
type ConsulClientSelector struct {
	ConsulAddress      string
	consulConfig       *api.Config
	client             *api.Client
	ticker             *time.Ticker
	sessionTimeout     time.Duration
	Servers            []*api.AgentService
	WeightedServers    []*Weighted
	ServiceName        string
	SelectMode         src.SelectMode
	dailTimeout        time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *src.Client
}

// NewConsulClientSelector creates a ConsulClientSelector
func NewConsulClientSelector(consulAddress string, serviceName string, sessionTimeout time.Duration, sm src.SelectMode, dailTimeout time.Duration) *ConsulClientSelector {
	selector := &ConsulClientSelector{
		ConsulAddress:  consulAddress,
		ServiceName:    serviceName,
		Servers:        make([]*api.AgentService, 1),
		sessionTimeout: sessionTimeout,
		SelectMode:     sm,
		dailTimeout:    dailTimeout,
		rnd:            rand.New(rand.NewSource(time.Now().UnixNano()))}

	selector.start()
	return selector
}

func (s *ConsulClientSelector) SetClient(c *src.Client) {
	s.Client = c
}

func (s *ConsulClientSelector) SetSelectMode(sm src.SelectMode) {
	s.SelectMode = sm
}

func (s *ConsulClientSelector) AllClients(clientCodecFunc src.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range s.Servers {
		ss := strings.Split(sv.Address, "@")
		c, err := src.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
		if err == nil {
			clients = append(clients, c)
		}
	}

	return clients
}

func (s *ConsulClientSelector) start() {
	if s.consulConfig == nil {
		s.consulConfig = api.DefaultConfig()
		s.consulConfig.Address = s.ConsulAddress
	}
	s.client, _ = api.NewClient(s.consulConfig)

	s.pullServers()

	s.ticker = time.NewTicker(s.sessionTimeout)
	go func() {
		for range s.ticker.C {
			s.pullServers()
		}
	}()
}

func (s *ConsulClientSelector) pullServers() {
	agent := s.client.Agent()
	ass, err := agent.Services()

	if err != nil {
		return
	}

	var services []*api.AgentService
	for k, v := range ass {
		if strings.HasPrefix(k, s.ServiceName) {
			services = append(services, v)
		}
	}
	s.Servers = services
}

func (s *ConsulClientSelector) createWeighted(ass map[string]*api.AgentService) {
	s.WeightedServers = make([]*Weighted, len(s.Servers))

	i := 0
	for k, v := range ass {
		if strings.HasPrefix(k, s.ServiceName) {
			s.WeightedServers[i] = &Weighted{Server: v, Weight: 1, EffectiveWeight: 1}
			i++
			if len(v.Tags) > 0 {
				if values, err := url.ParseQuery(v.Tags[0]); err == nil {
					w := values.Get("weight")
					if w != "" {
						weight, err := strconv.Atoi(w)
						if err != nil {
							s.WeightedServers[i].Weight = weight
							s.WeightedServers[i].EffectiveWeight = weight
						}
					}
				}
			}

		}
	}

}

//Select returns a rpc client
func (s *ConsulClientSelector) Select(clientCodecFunc src.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.len == 0 {
		return nil, errors.New("No available service")
	}

	if s.SelectMode == src.RandomSelect {
		s.currentServer = s.rnd.Intn(s.len)
		server := s.Servers[s.currentServer]
		ss := strings.Split(server.Address, "@") //tcp@ip , tcp4@ip or tcp6@ip
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)

	} else if s.SelectMode == src.RandomSelect {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		server := s.Servers[s.currentServer]
		ss := strings.Split(server.Address, "@")
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)

	} else if s.SelectMode == src.ConsistentHash {
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options)
		server := s.Servers[s.currentServer]
		ss := strings.Split(server.Address, "@")
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
	} else if s.SelectMode == src.WeightedRoundRobin {
		server := nextWeighted(s.WeightedServers).Server.(*api.AgentService)
		ss := strings.Split(server.Address, "@")
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, ss[0], ss[1], s.dailTimeout)
	}

	return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())

}
