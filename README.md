The application implements a peer-to-peer node using the Ricart-Agrawala algorithm for distributed mutual exclusion. Nodes can either start a new network or connect to an existing network and communi

# Running the Program
To run the program use the command:
`go run node/node.go ip port [existing_node_ip existing_node_port]`

Where `ip` and `port` is the ip and port of the node that is being created, and the optional `existing_node_ip` and `existing_node_port` is the ip and port of a node already integrated into the network.

## Example
1. Start the first node
`go run node/node.go localhost 5000`
This node will initialize the network and exist on `localhost` with port `5000`.

2. Start additional nodes (connect to an existing node):
```
go run node/node.go localhost 5001 localhost 5000
go run node/node.go localhost 5002 localhost 5001
```
Please note that when specifying the existing node, it is important that it knows all other nodes in the network. It will communicate via gossip about that the new node has joined.
