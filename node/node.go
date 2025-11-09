package main

import (
	proto "consensus-go/grpc"
	clock "consensus-go/clock"
	queue "consensus-go/queue"
	"context"

	"fmt"
	"log"
	"time"
	"os"
	"net"
	"math/rand"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)


type NodeState byte;
const (
	StateReleased NodeState = iota
	StateWanted
	StateHeld
)

type NodeInfo struct {
	ip string
	port uint16
}

type Node struct {
	proto.UnimplementedConsensusServiceServer

	state NodeState
	clock clock.AtomicLamportClock
	requestTimeStamp uint32

	nodeInfo NodeInfo
	knownNodes []NodeInfo
	queue queue.AtomicQueue[NodeInfo]

	accept chan bool
}

func NewNode(nodeInfo NodeInfo, knownNodes []NodeInfo) *Node {
	node := new(Node)

	node.state = StateReleased;
	node.clock = clock.NewClock();
	node.nodeInfo = nodeInfo;
	node.accept = make(chan bool, 1);

	node.knownNodes = knownNodes;

	grpcServer := grpc.NewServer()
	proto.RegisterConsensusServiceServer(grpcServer, node);

	connectionAddr := fmt.Sprintf("%s:%d", nodeInfo.ip, nodeInfo.port);
	tcpConnection, err := net.Listen("tcp", connectionAddr);

	if err != nil {
		panic("Failed to bind ")
	}

	go nodeLoop(node);
	grpcServer.Serve(tcpConnection);

	return node;
}

func (node *Node) Log(format string, v ...any) {
	log.Printf("%s:%d: %s\n", node.nodeInfo.ip, node.nodeInfo.port, fmt.Sprintf(format, v...));
}

func (node *Node) AcceptRequest(ctx context.Context, req *proto.Ok) (*proto.Nothing, error) {
	node.Log("My request was accepted by a node");

//	node.clock.Advance();
	node.clock.MergeWithRawTimestamp(req.Lamport);
	node.accept <- true

	return &proto.Nothing{}, nil
}

func (node *Node) RequestAccess(ctx context.Context, req *proto.Request) (*proto.Nothing, error) {
	node.clock.Advance()

	ip := req.Ip;
	port := uint16(req.Port);

	node.Log("Node (%s:%d) is asking for my permission to access the critical section", ip, port);

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", ip, port),
		grpc.WithTransportCredentials(insecure.NewCredentials()));

	if err != nil {
		println("err");
	}

	client := proto.NewConsensusServiceClient(conn);

	switch node.state {
	case StateReleased:
		node.Log("Don't need access myself, so I'll just accept the request immediately");
		client.AcceptRequest(
			ctx, 
			&proto.Ok{},
		);

	case StateWanted:
		if req.Lamport <= node.requestTimeStamp {
			node.Log("They asked for access before me, so I'll accept their request");
			client.AcceptRequest(
				context.Background(), 
				&proto.Ok{},
			);
		} else {
			node.Log("I asked for access before them, so I'll push their request to the queue");
			node.queue.Push( NodeInfo { ip: ip, port: port } );
		}

	case StateHeld:
			node.Log("I currently hold access to the critical section, so I'll just push their request to the queue");
			node.queue.Push( NodeInfo { ip: ip, port: port } );
	}

	node.clock.MergeWithRawTimestamp(req.Lamport);

	conn.Close();

	return &proto.Nothing{}, nil
}

func (node *Node) Enter() {
	n := len(node.knownNodes);
	node.Log("I'd like to access the critical section, let me ask the other %d nodes first", n);
	node.state = StateWanted;

	node.clock.Advance();
	node.requestTimeStamp = node.clock.CurrentTime();

	requestPacket := proto.Request{
		Lamport: node.clock.CurrentTime(),
		Ip: node.nodeInfo.ip,
		Port: uint32(node.nodeInfo.port),
	};

	for i := range n {
		nodeInfo := node.knownNodes[i];
		conn, err := grpc.NewClient(
			fmt.Sprintf("%s:%d", nodeInfo.ip, nodeInfo.port),
			grpc.WithTransportCredentials(insecure.NewCredentials()));

		if err != nil {
			node.Log("Could not ask node: (%s:%d)", nodeInfo.ip, nodeInfo.port);
			n -= 1
			continue;
		}

		client := proto.NewConsensusServiceClient(conn);

		node.Log("Sending request access to: (%s:%d)", nodeInfo.ip, nodeInfo.port);
		ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(time.Second*3));
		client.RequestAccess(ctx, &requestPacket);

		conn.Close();

	}

	node.Log("All requests have been sent, I'll now wait for %d replies", n);
	for n > 0 {
		node.Log("Waiting for %d replies from the other nodes", n);
		accepted := <-node.accept
		if accepted { n -= 1 }

	}

	node.state = StateHeld;
	node.Log("I now hold access to the critical section");

}

func (node *Node) Exit() {
	node.state = StateReleased;
	node.Log("I no longer want access to the critical section");

	for node.queue.Size() > 0 {
		rep := node.queue.Pop();

		conn, err := grpc.NewClient(
			fmt.Sprintf("%s:%d", rep.ip, rep.port),
			grpc.WithTransportCredentials(insecure.NewCredentials()));

		node.Log("Telling (%s:%d) that I'm all done and that they can access the critical section", rep.ip, rep.port);

		if err != nil {
			node.Log("Could not contact (%s:%d)", rep.ip, rep.port);
			continue;
		}

		client := proto.NewConsensusServiceClient(conn);

		client.AcceptRequest(context.Background(), &proto.Ok{});

		conn.Close();
	}
}

func nodeLoop(node *Node) {
	for {
		idleTime := (rand.Uint64() % 60) + 1;
		node.Log("Working on non-critical section for %d seconds", idleTime)
		time.Sleep(time.Duration(idleTime) * time.Second);

		node.Enter();

		idleTime = (rand.Uint64() % 20) + 1;
		node.Log("Working on critical section for %d seconds", idleTime)
		time.Sleep(time.Duration(idleTime) * time.Second);

		node.Exit();

	}

}

func main() {
	if len(os.Args) < 2 {
		println("Missing node arg: 'a', 'b' or 'c'");
		return;
	}

	println("Waiting 5s before startup");
	time.Sleep( 5*time.Second );
	println("Server starting");

	var node *Node;
	switch os.Args[1][0] {
	case 'a':

	knownNodes := []NodeInfo{
		NodeInfo {ip: "localhost", port: 5000},
		NodeInfo {ip: "localhost", port: 5001},
	};

	node = NewNode(NodeInfo {ip: "localhost", port: 5002}, knownNodes);
	case 'b':
	knownNodes := []NodeInfo{
		NodeInfo {ip: "localhost", port: 5001},
		NodeInfo {ip: "localhost", port: 5002},
	};

	node = NewNode(NodeInfo {ip: "localhost", port: 5000}, knownNodes);
	case 'c':
	knownNodes := []NodeInfo{
		NodeInfo {ip: "localhost", port: 5000},
		NodeInfo {ip: "localhost", port: 5002},
	};

	node = NewNode(NodeInfo {ip: "localhost", port: 5001}, knownNodes);
	}

	println(node.state);
}


