package clientselector

import (
	"errors"
	"math/rand"
	"net/rpc"
	"time"

	"../src"
)

// ServerPeer is
type ServerPeer struct {
	Network, Address string
	Weight           int
}

// MultiClientSelector is used to select a direct rpc server from a list.
type MultiClientSelector struct {
	Servers            []*ServerPeer
	WeightedServers    []*Weighted
	SelectMode         src.SelectMode
	dailTimeout        time.Duration
	rnd                *rand.Rand
	currentServer      int
	len                int
	HashServiceAndArgs HashServiceAndArgs
	Client             *src.Client
}

// NewMultiClientSelector creates a MultiClientSelector
func NewMultiClientSelector(servers []*ServerPeer, sm src.SelectMode, dailTimeout time.Duration) *MultiClientSelector {
	s := &MultiClientSelector{
		Servers:     servers,
		SelectMode:  sm,
		dailTimeout: dailTimeout,
		rnd:         rand.New(rand.NewSource(time.Now().UnixNano())),
		len:         len(servers)}

	if sm == src.WeightedRoundRobin {
		s.WeightedServers = make([]*Weighted, len(s.Servers))
		for i, ss := range s.Servers {
			s.WeightedServers[i] = &Weighted{Server: ss, Weight: ss.Weight, EffectiveWeight: ss.Weight}
		}
	}

	s.currentServer = s.rnd.Intn(s.len)
	return s
}

func (s *MultiClientSelector) SetClient(c *src.Client) {
	s.Client = c
}

func (s *MultiClientSelector) SetSelectMode(sm src.SelectMode) {
	s.SelectMode = sm
}

func (s *MultiClientSelector) AllClients(clientCodecFunc src.ClientCodecFunc) []*rpc.Client {
	var clients []*rpc.Client

	for _, sv := range s.Servers {
		c, err := src.NewDirectRPCClient(s.Client, clientCodecFunc, sv.Network, sv.Address, s.dailTimeout)
		if err == nil {
			clients = append(clients, c)
		}
	}

	return clients
}

//Select returns a rpc client
func (s *MultiClientSelector) Select(clientCodecFunc src.ClientCodecFunc, options ...interface{}) (*rpc.Client, error) {
	if s.len == 0 {
		return nil, errors.New("No available service")
	}

	if s.SelectMode == src.RandomSelect {
		s.currentServer = s.rnd.Intn(s.len)
		peer := s.Servers[s.currentServer]
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, peer.Network, peer.Address, s.dailTimeout)

	} else if s.SelectMode == src.RoundRobin {
		s.currentServer = (s.currentServer + 1) % s.len //not use lock for performance so it is not precise even
		peer := s.Servers[s.currentServer]
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, peer.Network, peer.Address, s.dailTimeout)
	} else if s.SelectMode == src.ConsistentHash {
		if s.HashServiceAndArgs == nil {
			s.HashServiceAndArgs = JumpConsistentHash
		}
		s.currentServer = s.HashServiceAndArgs(s.len, options...)
		peer := s.Servers[s.currentServer]
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, peer.Network, peer.Address, s.dailTimeout)
	} else if s.SelectMode == src.WeightedRoundRobin {
		best := nextWeighted(s.WeightedServers)
		peer := best.Server.(*ServerPeer)
		return src.NewDirectRPCClient(s.Client, clientCodecFunc, peer.Network, peer.Address, s.dailTimeout)
	}

	return nil, errors.New("not supported SelectMode: " + s.SelectMode.String())
}
