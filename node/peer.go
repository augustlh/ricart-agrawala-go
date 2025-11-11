package node

import (
	"log"

	pb "github.com/augustlh/ricart-agrawala-go/proto"
	"google.golang.org/grpc"
)

type Peer struct {
	id     string
	stream grpc.BidiStreamingClient[pb.Message, pb.Message]
	send   chan *pb.Message
}

func NewPeer(id string, stream grpc.BidiStreamingClient[pb.Message, pb.Message]) *Peer {
	p := &Peer{
		id:     id,
		stream: stream,
		send:   make(chan *pb.Message, 10),
	}
	go p.listen()
	return p
}

func (p *Peer) listen() {
	for msg := range p.send {
		if err := p.stream.Send(msg); err != nil {
			log.Printf("[%s] stream send error: %v", p.id, err)
			return
		}
	}
}

func (p *Peer) Send(msg *pb.Message) {
	select {
	case p.send <- msg:
	default:
		log.Printf("[%s] send channel full, dropping message", p.id)
	}
}
