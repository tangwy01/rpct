package main

import "./src"

type Args struct {
	A int `msg:"a"`
	B int `msg:"b"`
}

type Reply struct {
	C int `msg:"c"`
}

type Arith int

func (t *Arith) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B
	return nil
}

func (t *Arith) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

type Arith2 int

func (t *Arith2) Mul(args *Args, reply *Reply) error {
	reply.C = args.A * args.B * 10
	return nil
}

func (t *Arith2) Error(args *Args, reply *Reply) error {
	panic("ERROR")
}

func main() {
	server1 := src.NewServer()
	server1.RegisterName("Arith", new(Arith))
	server1.Start("tcp", "127.0.0.1:8972")

	server2 := src.NewServer()
	server2.RegisterName("Arith", new(Arith2))
	server2.Serve("tcp", "127.0.0.1:8973")
}
