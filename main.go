package main

import (
	"bufio"
	"log"
	"os"
	"strings"

	"github.com/augustlh/ricart-agrawala-go/node"
)

func Parse(path string) []*node.NodeInfo {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("Failed to open nodes file, please ensure it exists!")
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	nodes := []*node.NodeInfo{}
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")

		nodes = append(nodes, &node.NodeInfo{Id: parts[0], Ip: parts[1]})
		if err := scanner.Err(); err != nil {
			log.Fatalf("error reading file: %v", err)
		}
	}

	return nodes

}

func main() {
	path := "./nodes.txt"
	nodes := Parse(path)

	for _, n := range nodes {
		go node.NewNode(n.Id, n.Ip, nodes)
	}

	for {
	}
}
