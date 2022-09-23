package main

import (
	"os"
)

func main() {
	//입력한 포트번호
	nodeID := os.Args[1] /// cmd] pbft 5000 [enter]
	server := NewServer(nodeID)

	server.Start()
}
