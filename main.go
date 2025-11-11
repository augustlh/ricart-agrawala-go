package main

import (
	"context"
	"log"
	discovery "ricart_agrawala/Discovery"
	"time"

	capi "github.com/hashicorp/consul/api"
)

func main() {
	cfg := capi.DefaultConfig()
	d, err := discovery.NewConsul("ra", cfg)
	if err != nil {
		panic(err)
	}

	nodes := []*Node{
		NewNode(1, ":50051", nil),
		NewNode(2, ":50052", nil),
		NewNode(3, ":50053", nil),
	}
	for _, n := range nodes {
		if err := n.Start(); err != nil {
			panic(err)
		}
	}

	ctx := context.Background()
	if err := d.Register(ctx, 1, "127.0.0.1:50051"); err != nil {
		panic(err)
	}
	if err := d.Register(ctx, 2, "127.0.0.1:50052"); err != nil {
		panic(err)
	}

	go func() {
		for {
			peers, err := d.ListPeers(ctx)
			if err != nil {
				log.Printf("discover: %v", err)
			}
			for _, n := range nodes {
				n.SetPeers(peers)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}()

	for _, n := range nodes {
		go func(node *Node) {
			node.RequestCS(ctx)
			for !node.CanEnter() {
				time.Sleep(10 * time.Millisecond)
			}
			log.Printf("[Node %d] enter CS", node.id)
			time.Sleep(200 * time.Millisecond)
			node.ReleaseCS(ctx)
			log.Printf("[Node %d] exit CS", node.id)
		}(n)
	}

}
