package main

import (
	"context"
	"log"
	"net"
	"ricart_agrawala/lamport"
	pb "ricart_agrawala/proto"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Node struct {
	pb.UnimplementedRAServer
	id       int32
	addr     string
	peers    map[int32]string
	mu       sync.Mutex
	clock    lamport.Clock
	wantCS   bool
	reqTS    int64
	waiting  map[int32]bool
	deferred map[int32]bool
}

func NewNode(id int32, addr string, peers map[int32]string) *Node {
	return &Node{id: id, addr: addr, peers: peers, waiting: map[int32]bool{}, deferred: map[int32]bool{}}
}

func (n *Node) Start() error {
	lis, err := net.Listen("tcp", n.addr)
	if err != nil {
		return err
	}
	s := grpc.NewServer()
	pb.RegisterRAServer(s, n)
	log.Printf("[n%d] listening on %s", n.id, n.addr)
	go func() { _ = s.Serve(lis) }()
	return nil
}

func (n *Node) RequestCS(ctx context.Context) {
	ts := n.clock.Tick()
	n.mu.Lock()
	n.wantCS = true
	n.reqTS = ts
	n.waiting = make(map[int32]bool, len(n.peers))
	for pid := range n.peers {
		if pid != n.id {
			n.waiting[pid] = true
		}
	}
	peers := make(map[int32]string, len(n.peers))
	for k, v := range n.peers {
		peers[k] = v
	}
	n.mu.Unlock()

	req := &pb.RequestMsg{From: n.id, Ts: ts}
	for pid, addr := range peers {
		if pid == n.id {
			continue
		}
		go n.sendRequest(ctx, addr, req)
	}
}

func (n *Node) CanEnter() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.wantCS {
		return false
	}
	for _, pending := range n.waiting {
		if pending {
			return false
		}
	}
	return true
}

func (n *Node) ReleaseCS(ctx context.Context) {
	n.mu.Lock()
	n.wantCS = false
	n.reqTS = 0
	deferList := make([]int32, 0, len(n.deferred))
	for pid, d := range n.deferred {
		if d {
			deferList = append(deferList, pid)
		}
	}
	n.deferred = map[int32]bool{}
	peers := make(map[int32]string, len(n.peers))
	for k, v := range n.peers {
		peers[k] = v
	}
	n.mu.Unlock()
	for _, pid := range deferList {
		if addr, ok := peers[pid]; ok {
			go n.sendReply(ctx, addr, &pb.ReplyMsg{From: n.id})
		}
	}
}

func (n *Node) Request(ctx context.Context, m *pb.RequestMsg) (*pb.Ack, error) {
	n.clock.Observe(m.Ts)
	n.mu.Lock()
	defer n.mu.Unlock()
	shouldDefer := n.wantCS && !less(m.Ts, m.From, n.reqTS, n.id)
	if shouldDefer {
		n.deferred[m.From] = true
		log.Printf("[n%d] defer reply to %d", n.id, m.From)
		return &pb.Ack{Ok: true}, nil
	}
	addr, ok := n.peers[m.From]
	if ok {
		go n.sendReply(context.Background(), addr, &pb.ReplyMsg{From: n.id})
	}
	return &pb.Ack{Ok: true}, nil
}

func (n *Node) Reply(ctx context.Context, m *pb.ReplyMsg) (*pb.Ack, error) {
	_ = n.clock.Tick()
	n.mu.Lock()
	delete(n.waiting, m.From)
	n.mu.Unlock()
	return &pb.Ack{Ok: true}, nil
}

func (n *Node) sendRequest(ctx context.Context, addr string, req *pb.RequestMsg) {

	conn, err := grpc.NewClient(
		"dns:///localhost:5050",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("[n%d] dial %s: %v", n.id, addr, err)
		return
	}
	defer conn.Close()
	cli := pb.NewRAClient(conn)
	if _, err := cli.Request(ctx, req); err != nil {
		log.Printf("[n%d] Request->%s err: %v", n.id, addr, err)
	}
}

func (n *Node) sendReply(ctx context.Context, addr string, rep *pb.ReplyMsg) {

	conn, err := grpc.NewClient(
		"dns:///localhost:5050",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("[n%d] dial %s: %v", n.id, addr, err)
		return
	}
	defer conn.Close()
	cli := pb.NewRAClient(conn)
	if _, err := cli.Reply(ctx, rep); err != nil {
		log.Printf("[n%d] Reply->%s err: %v", n.id, addr, err)
	}
}

func less(tsA int64, pidA int32, tsB int64, pidB int32) bool {
	if tsA != tsB {
		return tsA < tsB
	}
	return pidA < pidB
}

func (n *Node) SetPeers(peers map[int32]string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.peers = peers
}
