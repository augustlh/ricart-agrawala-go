package node

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/augustlh/ricart-agrawala-go/clock"
	pb "github.com/augustlh/ricart-agrawala-go/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	num_nodes int
)

type NodeInfo struct {
	Id string
	Ip string
}

type State int

const (
	Released State = iota
	Held
	Wanted
)

type Node struct {
	pb.UnimplementedRicartAgrawalaServiceServer
	id string
	ip string

	mu    sync.Mutex
	clock clock.LamportClock
	state State

	recv  chan *pb.Reply
	peers map[string]*Peer
	queue []*pb.Request

	wantedTimestamp uint64 // The timestamp of when the node starting wanting to enter the critical section
}

func (n *Node) Stream(stream pb.RicartAgrawalaService_StreamServer) error {
	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Printf("[%s] stream closed", n.id)
				return nil
			}
			log.Printf("[%s] stream recv error: %v", n.id, err)
			return err
		}

		switch in.Payload.(type) {
		case *pb.Message_Reply:
			log.Printf("[%s] received reply: %v", n.id, in.GetReply())
			n.recv <- in.GetReply()
		case *pb.Message_Request:
			req := in.GetRequest()
			log.Printf("[%s] received request: %v", n.id, in.GetRequest())

			n.mu.Lock()
			n.clock.Sync(req.Timestamp)
			// If we have an active wanted, we check if the wantedTimestamp < req.Timestamp, and if it is we queue it since we have prio
			if n.state == Held || (n.state == Wanted && (n.wantedTimestamp < req.Timestamp || n.wantedTimestamp == req.Timestamp && n.id < req.Id)) {
				n.queue = append(n.queue, req)
				n.mu.Unlock()
			} else {
				n.SendReply(req)
				n.mu.Unlock()
			}
		}
	}
}

func (n *Node) Multicast(message *pb.Message) {
	log.Printf("[%s] multicasting: %v", n.id, message)
	n.mu.Lock()
	peers := make([]*Peer, 0, len(n.peers))
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	n.mu.Unlock()

	for _, peer := range peers {
		peer.Send(message)
	}
}

func (n *Node) Enter() {
	n.mu.Lock()
	if n.state != Released {
		n.mu.Unlock()
		return
	}
	log.Printf("[%v] is trying to access the critical section", n.id)
	n.state = Wanted

	n.clock.Tick()
	wanted := n.clock.Now()
	n.wantedTimestamp = wanted
	n.mu.Unlock()

	request := pb.Message{
		Payload: &pb.Message_Request{
			Request: &pb.Request{
				Id:        n.id,
				Timestamp: n.wantedTimestamp,
			},
		},
	}

	//multicast it to all nodes
	n.Multicast(&request)

	//now we want n-1 responses
	acks := make(map[string]bool)

	for len(acks) < num_nodes-1 {
		select {
		case msg := <-n.recv:
			if msg.RequestTimestamp == wanted {
				acks[msg.Id] = true
			} else {
				log.Printf("[%s] ignoring reply for ts=%d (wanted=%d)", n.id, msg.RequestTimestamp, wanted)
			}
		}
		log.Printf("[%v] %v", n.id, acks)
	}

	n.mu.Lock()
	n.state = Held
	n.mu.Unlock()
}

func (n *Node) Exit() {
	n.mu.Lock()
	if n.state != Held {
		n.mu.Unlock()
		return
	}

	log.Printf("[%s] exited the critcal section", n.id)

	n.state = Released
	n.recv = make(chan *pb.Reply, num_nodes)
	queue := make([]*pb.Request, len(n.queue))
	copy(queue, n.queue)
	n.mu.Unlock()

	for _, req := range queue {
		n.SendReply(req)
	}
}

func (n *Node) SendReply(req *pb.Request) {
	n.mu.Lock()
	p := n.peers[req.Id]
	n.mu.Unlock()
	reply := &pb.Message{
		Payload: &pb.Message_Reply{
			Reply: &pb.Reply{
				Id:               n.id,
				RequestTimestamp: req.Timestamp,
			},
		},
	}
	p.Send(reply)
}

type AccessResult int

const (
	Refused AccessResult = iota
	Success
)

// Access simulates a node entering its critical section by printing to `stdout`.
func (n *Node) Access() AccessResult {
	// This naturally assumes that their are no byzantine faults
	// If a system became rogue, it could just lie and say it holds the token even when it doesn't
	n.mu.Lock()
	if n.state == Held {
		n.mu.Unlock()
		duration := 1 //rand.Float64()
		log.Printf("[%v] has accessed the critical section and will use it for %d seconds\n", n.id, duration)
		time.Sleep(time.Duration(duration) * time.Second)
		return Success
	}

	return Refused

}

// Connect the `Node` to a list of servers
func (n *Node) Connect(nodes []*NodeInfo) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	for _, other := range nodes {
		if n.id == other.Id || n.ip == other.Ip {
			continue
		}

		for {
			conn, err := grpc.NewClient(other.Ip, opts...)
			if err != nil {
				log.Printf("fail to dial: %v", err)
				time.Sleep(500 * time.Millisecond)
				continue
			}

			client := pb.NewRicartAgrawalaServiceClient(conn)

			stream, err := client.Stream(context.Background())
			if err != nil {
				log.Printf("failed to create stream with server: %v", err)
				conn.Close()
				time.Sleep(500 * time.Millisecond)
				continue
			}

			p := NewPeer(other.Id, stream)
			n.mu.Lock()
			n.peers[other.Id] = p
			n.mu.Unlock()
			break
		}
	}

	go n.Act()
}

func NewNode(id string, ip string, nodes []*NodeInfo) {
	num_nodes = len(nodes)
	n := &Node{
		id:              id,
		ip:              ip,
		clock:           clock.NewClock(),
		state:           Released,
		recv:            make(chan *pb.Reply, 10),
		peers:           make(map[string]*Peer),
		queue:           []*pb.Request{},
		wantedTimestamp: 0,
	}

	fmt.Printf("[%v] listen called with ip=%v\n", n.id, n.ip)
	lis, err := net.Listen("tcp", ip)
	if err != nil {
		log.Fatalf("[%s] failed to listen: %v", n.id, err)
	}
	log.Printf("[%s] now listening at %v", n.id, n.ip)

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	pb.RegisterRicartAgrawalaServiceServer(grpcServer, n)
	go grpcServer.Serve(lis)

	go n.Connect(nodes)

}

// Makes the `Node` act
// It tries to access the critical section, the acts 0..n many times for n <= 10 and releases the lock which repeats
func (n *Node) Act() {
	log.Printf("[%v] started acting", n.id)
	for {
		n.Enter()
		n.Access()
		n.Exit()

		time.Sleep(1 * time.Second)
	}
}
