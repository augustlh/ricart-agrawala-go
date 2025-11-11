package discovery

import (
	"context"
	"fmt"

	capi "github.com/hashicorp/consul/api"
)

type Discovery interface {
	Register(ctx context.Context, pid int32, addr string) error
	Deregister(ctx context.Context, pid int32) error
	ListPeers(ctx context.Context) (map[int32]string, error)
}

type Consul struct {
	client *capi.Client
	name   string
}

func NewConsul(name string, cfg *capi.Config) (*Consul, error) {
	c, err := capi.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Consul{client: c, name: name}, nil
}

func (c *Consul) Register(ctx context.Context, pid int32, addr string) error {
	reg := &capi.AgentServiceRegistration{
		ID:      fmt.Sprintf("%s-%d", c.name, pid),
		Name:    c.name,
		Address: host(addr),
		Port:    port(addr),
		Tags:    []string{fmt.Sprintf("pid=%d", pid)},
	}
	return c.client.Agent().ServiceRegister(reg)
}

func (c *Consul) Deregister(ctx context.Context, pid int32) error {
	return c.client.Agent().ServiceDeregister(fmt.Sprintf("%s-%d", c.name, pid))
}

func (c *Consul) ListPeers(ctx context.Context) (map[int32]string, error) {
	out := map[int32]string{}
	entries, _, err := c.client.Health().Service(c.name, "", true, nil)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		pid := parsePID(e.Service.Tags)
		if pid == 0 {
			continue
		}
		out[pid] = fmt.Sprintf("%s:%d", e.Service.Address, e.Service.Port)
	}
	return out, nil
}

func host(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

func port(addr string) int {
	p := 0
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			for _, ch := range addr[i+1:] {
				p = p*10 + int(ch-'0')
			}
			return p
		}
	}
	return 0
}

func parsePID(tags []string) int32 {
	var id int
	for _, t := range tags {
		if len(t) > 4 && t[:4] == "pid=" {
			fmt.Sscanf(t, "pid=%d", &id)
			return int32(id)
		}
	}
	return 0
}
