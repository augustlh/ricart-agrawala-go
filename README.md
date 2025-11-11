# Ricart-Agrawala-Go
The application implements a peer-to-peer node using the Ricart-Agrawala algorithm for distributed mutual exclusion. 

### Warning
Please note that the implementation assumes that there is a reliable message delivery and no process failures (even thought this obviously is impossible in the real world).

## Running the Program
To run the program use the command:
```
go run node/node.go ip port [existing_node_ip existing_node_port]
```

Where `ip` and `port` is the ip and port of the node that is being created, and the optional `existing_node_ip` and `existing_node_port` is the ip and port of a node already integrated into the network. Please note that when specifying the existing node, it is important that it knows all other nodes in the network. This allows the new node to gossip its existence to the rest of the network through the existing node.

### Example
1. Start the first node
```
go run node/node.go localhost 5000
```
This node will initialize the network and exist on `localhost` with port `5000`.

2. Start additional nodes (connect to an existing node):
```
go run node/node.go localhost 5001 localhost 5000
go run node/node.go localhost 5002 localhost 5001
```

## Contributors
The application is developed in cooperation between `augustlh`, `RockRottenSalad` and `KumaSC`

## License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
