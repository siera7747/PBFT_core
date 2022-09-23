package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Server struct {
	url      string
	node     *Node
	ErrorNum int
}

//새 서버 생성. 노드 생성하고 해당 노드에 해당하는 서버를 생성한 다음 기본 설정을 마친다.
func NewServer(nodeID string) *Server {
	node := NewNode(nodeID)
	server := &Server{node.NodeAddressTable[nodeID], node, 0}

	server.setRoute()

	return server
}

//서버 스타트
func (server *Server) Start() {
	fmt.Printf("Server will be started at %s...\n", server.url)
	if err := http.ListenAndServe(server.url, nil); err != nil {
		fmt.Println(err)
		return
	}
}

//서버 주소와 함수 연결
func (server *Server) setRoute() {
	http.HandleFunc("/pbft", server.getReq) //Leader Node
	http.HandleFunc("/preprepare", server.getPrePrepare)
	http.HandleFunc("/prepare", server.getPrepare)
	http.HandleFunc("/commit", server.getCommit)
	http.HandleFunc("/reply", server.getReply) //Leader Node ==> Http Server 주소변경
	http.HandleFunc("/error", server.getError)
	http.HandleFunc("/addblock", server.getAddBlock)
}

//여기부터는 다른 포트에서 오는 메세지를 받는 코드들

//리퀘스트 받음 - json 메세지 받아서 디코드한 다음 출력
func (server *Server) getReq(writer http.ResponseWriter, request *http.Request) {
	fmt.Println("Get reqMsg")
	var msg RequestMsg
	err := json.NewDecoder(request.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err)
		return
	}
	server.node.MsgEntrance <- &msg
}

func (server *Server) getPrePrepare(writer http.ResponseWriter, request *http.Request) {
	var msg PrePrepareMsg
	err := json.NewDecoder(request.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err)
		return
	}

	server.node.MsgEntrance <- &msg
}

func (server *Server) getPrepare(writer http.ResponseWriter, request *http.Request) {
	var msg VoteMsg
	err := json.NewDecoder(request.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err)
		return
	}

	server.node.MsgEntrance <- &msg
}

func (server *Server) getCommit(writer http.ResponseWriter, request *http.Request) {
	var msg VoteMsg
	err := json.NewDecoder(request.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err)
		return
	}

	server.node.MsgEntrance <- &msg
}

func (server *Server) getReply(writer http.ResponseWriter, request *http.Request) {
	var msg RequestMsg
	err := json.NewDecoder(request.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err)
		return
	}

	server.node.MsgEntrance <- &msg
}

func (server *Server) getError(writer http.ResponseWriter, request *http.Request) {
	var msg ReplyMsg
	err := json.NewDecoder(request.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Error. block delete.\n")
	server.node.DeleteBlock(&msg)
}

func (server *Server) getAddBlock(writer http.ResponseWriter, request *http.Request) {
	var msg RequestMsg
	fmt.Println("addblock server start")
	err := json.NewDecoder(request.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err)
		return
	}

	server.node.BlockAdd(&msg)
}

func send(url string, msg []byte) {
	buff := bytes.NewBuffer(msg)
	http.Post("http://"+url, "application/json", buff)
}
