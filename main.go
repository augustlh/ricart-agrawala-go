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

	A := NewNode(1, ":50051", nil)
	B := NewNode(2, ":50052", nil)

	if err := A.Start(); err != nil {
		panic(err)
	}
	if err := B.Start(); err != nil {
		panic(err)
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
			A.SetPeers(peers)
			B.SetPeers(peers)
			time.Sleep(300 * time.Millisecond)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	A.RequestCS(ctx)
	time.Sleep(50 * time.Millisecond)
	B.RequestCS(ctx)

	for !A.CanEnter() {
		time.Sleep(10 * time.Millisecond)
	}
	log.Println("[A] enter CS")
	time.Sleep(200 * time.Millisecond)
	A.ReleaseCS(ctx)
	log.Println("[A] exit CS")

	for !B.CanEnter() {
		time.Sleep(10 * time.Millisecond)
	}
	log.Println("[B] enter CS")
	time.Sleep(200 * time.Millisecond)
	B.ReleaseCS(ctx)
	log.Println("[B] exit CS")
}
