package main

import (
	"fmt"
	"time"

	"./src"
	"./clientselector"
)

type Args struct {
	A int `msg:"a"`
	B int `msg:"b"`
}

type Reply struct {
	C int `msg:"c"`
}

func main() {
	server1 := &clientselector.ServerPeer{Network: "tcp", Address: "127.0.0.1:8972"}
	server2 := &clientselector.ServerPeer{Network: "tcp", Address: "127.0.0.1:8973"}

	servers := []*clientselector.ServerPeer{server1, server2}

	s := clientselector.NewMultiClientSelector(servers, src.RandomSelect, 10*time.Second)

	for i := 0; i < 10; i++ {
		callServer(s)
	}
}

func callServer(s src.ClientSelector) {
	client := src.NewClient(s)

	args := &Args{7, 8}
	var reply Reply
	err := client.Call("Arith.Mul", args, &reply)
	if err != nil {
		fmt.Printf("error for Arith: %d*%d, %v \n", args.A, args.B, err)
	} else {
		fmt.Printf("Arith: %d*%d=%d \n", args.A, args.B, reply.C)
	}

	client.Close()
}
