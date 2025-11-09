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
	"strconv"
	"net"
	"math/rand"
	"sync/atomic"

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

type NodeConnection struct {
	nodeInfo NodeInfo
	connection proto.ConsensusServiceClient
}

type Node struct {
	proto.UnimplementedConsensusServiceServer

	state NodeState
	clock clock.AtomicLamportClock
	requestTimeStamp uint32

	nodeInfo NodeInfo
	knownNodes map[NodeInfo]proto.ConsensusServiceClient
	queue queue.AtomicQueue[NodeInfo]

	gossipOrigin atomic.Uint32
	accepted atomic.Uint32
}

func NewNode(nodeInfo NodeInfo, knownNodes []NodeInfo) *Node {
	node := new(Node)

	node.state = StateReleased;
	node.clock = clock.NewClock();
	node.nodeInfo = nodeInfo;

	if len(knownNodes) > 0 {
		fmt.Printf("%v\n", knownNodes[0]);
	}

	node.knownNodes = make(map[NodeInfo]proto.ConsensusServiceClient);
	for _, n := range knownNodes {
		if !node.addNode(n) {
			node.Log("Failed to add: (%s:%d)", n.ip, n.port);
		}
	}

	grpcServer := grpc.NewServer()
	proto.RegisterConsensusServiceServer(grpcServer, node);

	connectionAddr := fmt.Sprintf("%s:%d", nodeInfo.ip, nodeInfo.port);
	tcpConnection, err := net.Listen("tcp", connectionAddr);

	if err != nil {
		panic(fmt.Sprintf("Server failed to bind to port: %d", nodeInfo.port));
	}

	go grpcServer.Serve(tcpConnection);

	for info, client := range node.knownNodes {
		node.Log("Gossiping to: (%s:%d) about self", info.ip, info.port);
		client.Gossip(context.Background(), &proto.GossipInfo { 
			Lamport: node.clock.CurrentTime(),
			Ip: node.nodeInfo.ip,
			Port: uint32(node.nodeInfo.port),

			SenderIp: node.nodeInfo.ip,
			SenderPort: uint32(node.nodeInfo.port),
		});
	}

	return node;
}

func (node *Node) Log(format string, v ...any) {
	log.Printf("%s:%d: %s\n", node.nodeInfo.ip, node.nodeInfo.port, fmt.Sprintf(format, v...));
}

func (node *Node) addNode(nodeInfo NodeInfo) bool {
	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", nodeInfo.ip, nodeInfo.port),
		grpc.WithTransportCredentials(insecure.NewCredentials()));

	if err != nil {
		node.Log("Error connecting to known node (%s:%d)", nodeInfo.ip, nodeInfo.port);
		return false;
	}

	client := proto.NewConsensusServiceClient(conn);

	node.knownNodes[nodeInfo] = client;
	return true;
}

func (node *Node) Gossip(ctx context.Context, req *proto.GossipInfo) (*proto.Nothing, error) {
	node.Log("Heard some gossip about a new node called (%s:%d)", req.Ip, req.Port);

	if req.Ip == node.nodeInfo.ip && req.Port == uint32(node.nodeInfo.port) {
		node.Log("That gossip is about me, I'll just ignore it");
		return &proto.Nothing{}, nil
	}

	node.clock.MergeWithRawTimestamp(req.Lamport);
	newNodeInfo := NodeInfo {ip: req.Ip, port: uint16(req.Port)};

	_, alreadyKnow := node.knownNodes[newNodeInfo];

	if alreadyKnow {
		node.Log("I actually already know about (%s:%d)", req.Ip, req.Port);
		return &proto.Nothing{}, nil
	} else if !node.addNode(newNodeInfo) {
		node.Log("Could not connect to the new node (%s:%d)", req.Ip, req.Port);
		return &proto.Nothing{}, nil
	}

	client, _ := node.knownNodes[newNodeInfo];

	for otherNodeInfo, otherNodeClient := range node.knownNodes {

		if otherNodeInfo.ip == req.SenderIp && otherNodeInfo.port == uint16(req.SenderPort) {
			continue;
		}

		otherNodeClient.Gossip(context.Background(), &proto.GossipInfo{
			Lamport: node.clock.CurrentTime(),
			Ip: req.Ip,
			Port: req.Port,
			SenderIp: node.nodeInfo.ip,
			SenderPort: uint32(node.nodeInfo.port),
		});

	}

	client.Gossip(context.Background(), &proto.GossipInfo{
		Lamport: node.clock.CurrentTime(),
		Ip: node.nodeInfo.ip,
		Port: uint32(node.nodeInfo.port),
		SenderIp: node.nodeInfo.ip,
		SenderPort: uint32(node.nodeInfo.port),
	});

	return &proto.Nothing{}, nil
}

func (node *Node) AcceptRequest(ctx context.Context, req *proto.Ok) (*proto.Nothing, error) {
	node.Log("My request was accepted by a node");

//	node.clock.Advance();
	node.clock.MergeWithRawTimestamp(req.Lamport);
	node.accepted.Add(1);

	return &proto.Nothing{}, nil
}

func (node *Node) RequestAccess(ctx context.Context, req *proto.Request) (*proto.Nothing, error) {
	node.clock.Advance()

	ip := req.Ip;
	port := uint16(req.Port);
	client, foundNode := node.knownNodes[NodeInfo {ip: ip, port: port}];

	if(!foundNode) {
		node.Log("Got request from unknown node: (%s:%d)", req.Ip, req.Port);
		return &proto.Nothing{}, nil;
	}

	node.Log("Node (%s:%d) is asking for my permission to access the critical section", ip, port);

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

	return &proto.Nothing{}, nil
}

func (node *Node) Enter() {
	n := len(node.knownNodes);
	node.accepted.Store(0);

	node.Log("I'd like to access the critical section, let me ask the other %d nodes first", n);
	node.state = StateWanted;

	node.clock.Advance();
	node.requestTimeStamp = node.clock.CurrentTime();

	requestPacket := proto.Request{
		Lamport: node.clock.CurrentTime(),
		Ip: node.nodeInfo.ip,
		Port: uint32(node.nodeInfo.port),
	};

	for nodeInfo, client := range node.knownNodes {

		node.Log("Sending request access to: (%s:%d)", nodeInfo.ip, nodeInfo.port);
		client.RequestAccess(context.Background(), &requestPacket);
	}

	node.Log("All requests have been sent, I'll now wait for %d replies", n);
	for node.accepted.Load() < uint32(len(node.knownNodes)) {
		// Wait for all other nodes to accept the request
	}

	node.state = StateHeld;
	node.Log("I now hold access to the critical section");
}

func (node *Node) Exit() {
	node.state = StateReleased;
	node.Log("I no longer want access to the critical section");

	for node.queue.Size() > 0 {
		rep := node.queue.Pop();
		client, found := node.knownNodes[rep];
		if !found {
			node.Log("Unknown node requested access: (%s:%d)", rep.ip, rep.port);
			continue;
		}
		client.AcceptRequest(context.Background(), &proto.Ok{});
	}
}

func nodeLoop(node *Node) {
	for {
		idleTime := (rand.Uint64() % 20) + 1;
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
	if len(os.Args) < 3 {
		println("Missing args: 'ip' 'port' 'otherIp?' 'otherPort?'");
		return;
	}

	println("Server starting");

	portNumber, _ := strconv.ParseUint(os.Args[2], 10, 16);

	var node *Node;
	if len(os.Args) == 3 {
		node = NewNode( NodeInfo{ ip: os.Args[1], port: uint16(portNumber) }, []NodeInfo{} );
	} else {
		otherPort, _ := strconv.ParseUint(os.Args[4], 10, 16);
		node = NewNode( NodeInfo{ ip: os.Args[1], port: uint16(portNumber) }, []NodeInfo{  {ip: os.Args[3], port: uint16(otherPort)} } );
	}


	nodeLoop(node);
}

