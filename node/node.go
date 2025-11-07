package node

import clock "consensus-go/clock"
import grpc "consensus-go/grpc"
import "context"

type NodeState byte;
const (
	StateReleased NodeState = iota
	StateWanted
	StateHeld
)

type KnownNode struct {
	ip string
	port uint16
}

type Node struct {
	server grpc.UnimplementedConsensusServiceServer

	state NodeState
	clock clock.LamportClock

	ch chan grpc.Event

	knownNodes []KnownNode
}

func NewNode(knownNodes []KnownNode) *Node {
	node := new(Node)

	node.state = StateReleased;
	node.clock = clock.NewClock();
	node.ch = make(chan bool);

	node.knownNodes = knownNodes;

	go node.run();
	
	return node;
}

func (node *Node) AcceptRequest(ctx context.Context, req *grpc.Event) (error) {
	node.ch <- *req
	return nil
}

func (node *Node) RequestAccess(ctx context.Context, req *grpc.Event) (*grpc.Event, error) {
	node.ch <- *req
	return nil, nil
}

func (node *Node) Enter() {

}

func (node *Node) Exit() {

}

func (node *Node) want() {

}

func (node *Node) run() {
	for {

		select {
			
		}

	}
}



