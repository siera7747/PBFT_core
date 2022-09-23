package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Node struct {
	NodeID           string
	NodeAddressTable map[string]string // key=nodeID, value=url
	View             *View
	CurrentState     *State
	CommittedMsgs    []*RequestMsg // kinda block.
	MsgBuffer        *MsgBuffer
	MsgEntrance      chan interface{}
	MsgDelivery      chan interface{}
	Alarm            chan bool
	Flag             int //초기화 여부 확인
	Blockchain       *Blockchain
}

type MsgBuffer struct {
	ReqMsgs        []*RequestMsg
	PrePrepareMsgs []*PrePrepareMsg
	PrepareMsgs    []*VoteMsg
	CommitMsgs     []*VoteMsg
}

type View struct {
	ID      int64
	Primary string
	Reply   string
}

const ResolvingTimeDuration = time.Millisecond * 1000 // 1 second.

const ReplyUrl = "192.168.10.24:83/reply"

//
// cmd> exe 5000 [enter] <---- 5000 = nodeID
// exe파일 포트번호 입력해서 동작
//새 노드 생성
func NewNode(nodeID string) *Node {
	const viewID = 10000000000 // temporary.

	node := &Node{
		// Hard-coded for test.
		//노드 아이디, 포트번호. 테스트용이라 직접 입력함
		NodeID: nodeID,
		NodeAddressTable: map[string]string{
			"P1": "192.168.10.57:4000",
			"P2": "192.168.10.57:4001",
			"P3": "192.168.10.57:5000",
			"P4": "192.168.10.57:5001",
			"P5": "192.168.10.57:6000",
		},
		//primary = 리더노드
		View: &View{
			ID:      viewID,
			Primary: "P1",
			Reply:   "P5",
		},

		//구조체 초기화
		// Consensus-related struct
		CurrentState:  nil,
		CommittedMsgs: make([]*RequestMsg, 0),
		MsgBuffer: &MsgBuffer{
			ReqMsgs:        make([]*RequestMsg, 0),
			PrePrepareMsgs: make([]*PrePrepareMsg, 0),
			PrepareMsgs:    make([]*VoteMsg, 0),
			CommitMsgs:     make([]*VoteMsg, 0),
		},

		// Channels
		MsgEntrance: make(chan interface{}, 3),
		MsgDelivery: make(chan interface{}, 3),
		Alarm:       make(chan bool),
		Flag:        1,
	}

	//블록체인 생성
	node.Blockchain = NewBlockchain()

	if node.NodeID == node.View.Reply {
		node.Flag = -1
	}

	// Start message dispatcher
	// 메세지가 서버에서 전달되면 받기 위한 고루프
	go node.dispatchMsg()

	// Start alarm trigger
	go node.alarmToDispatcher()

	// Start message resolver
	go node.resolveMsg(&sync.Mutex{})

	return node
}

//전파
func (node *Node) Broadcast(msg interface{}, path string) map[string]error {
	errorMap := make(map[string]error)

	for nodeID, url := range node.NodeAddressTable {
		if nodeID == node.NodeID {
			continue
		}

		if nodeID == node.View.Reply {
			continue
		}

		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			errorMap[nodeID] = err
			continue
		}

		send(url+path, jsonMsg)
	}

	if len(errorMap) == 0 {
		return nil
	} else {
		return errorMap
	}
}

// 응답
func (node *Node) Reply(msg *RequestMsg) error {
	// Print all committed messages.
	for _, value := range node.CommittedMsgs {
		fmt.Printf("Committed value: %v, %d, AddBlock, %d\n", value.Hash, value.Height, value.SequenceID)
	}
	fmt.Print("\n")

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	send(node.NodeAddressTable[node.View.Reply]+"/reply", jsonMsg)

	return nil
}

func (node *Node) BlockAdd(reqMsg *RequestMsg) {
	fmt.Println("Addblock start")
	block := &Block{
		Hash:          reqMsg.Hash,
		PervBlockHash: reqMsg.PervBlockHash,
		Timestamp:     reqMsg.Timestamp,
		Pow:           reqMsg.Pow,
		Nonce:         reqMsg.Nonce,
		Bits:          reqMsg.Bits,
		TxId:          reqMsg.TxId,
		Height:        reqMsg.Height,
	}
	node.Blockchain.AddBlock(block)
	if node.NodeID != node.View.Reply {
		//응답노드 제외 초기화
		fmt.Printf("addblock complete\n")
		node.Flag = 0
		node.InitState()

		//리더노드는 새 합의 시작
		if node.NodeID == node.View.Primary {
			node.CheckReqMsg()
		} else {
			if len(node.MsgBuffer.PrePrepareMsgs) != 0 {
				for _, v := range node.MsgBuffer.PrePrepareMsgs {
					if v.RequestMsg.Height == node.Blockchain.Blocks[len(node.Blockchain.Blocks)-1].Height+1 {
						node.GetPrePrepare(v)
					}
				}
			}
		}
	}
}

// GetReq can be called when the node's CurrentState is nil.
// Consensus start procedure for the Primary.
// 리퀘스트 처리
func (node *Node) GetReq(reqMsg *RequestMsg) error {
	LogMsg(reqMsg)

	// Create a new state for the new
	err := node.createStateForNewConsensus()
	if err != nil {
		return err
	}

	// Start the consensus process.
	// 합의 진행 시작
	prePrepareMsg, err := node.CurrentState.StartConsensus(reqMsg)
	if err != nil {
		return err
	}

	LogStage(fmt.Sprintf("Consensus Process (ViewID:%d)", node.CurrentState.ViewID), false)

	// Send getPrePrepare message
	if prePrepareMsg != nil {
		node.Broadcast(prePrepareMsg, "/preprepare")
		LogStage("Pre-prepare", true)
	} else {
		// node.Flag = 0
		// fmt.Printf("Flag change from GetReq\n")
		return err
	}

	return nil
}

// GetPrePrepare can be called when the node's CurrentState is nil.
// Consensus start procedure for normal participants.
func (node *Node) GetPrePrepare(prePrepareMsg *PrePrepareMsg) error {

	LogMsg(prePrepareMsg)

	// Create a new state for the new
	err := node.createStateForNewConsensus()
	if err != nil {
		return err
	}

	prePareMsg, err := node.CurrentState.PrePrepare(prePrepareMsg, node.Blockchain)
	if err != nil {
		// node.Flag = 0
		// fmt.Printf("Flag change from GetPrepare\n")
		// send("192.168.10.57:4000/reply", nil)
		return err
	}

	if prePareMsg != nil {
		// Attach node ID to the message
		prePareMsg.NodeID = node.NodeID

		LogStage("Pre-prepare", true)
		node.Broadcast(prePareMsg, "/prepare")
		LogStage("Prepare", false)
	}

	return nil
}

// 합의 대기를 마치고 합의 시작 상태 돌입
func (node *Node) GetPrepare(prepareMsg *VoteMsg) error {
	LogMsg(prepareMsg)

	if node.CurrentState == nil {
		return nil
	}

	if node.CurrentState.CurrentStage == PrePrepared {
		commitMsg, err := node.CurrentState.Prepare(prepareMsg, node.Blockchain)
		if err != nil {
			// node.Flag = 0
			// fmt.Printf("Flag change from GetPrepare\n")
			// send("192.168.10.57:4000/reply", nil)
			return err
		}

		if commitMsg != nil {
			// Attach node ID to the message(누가 합의 대기를 마쳤는지 알려주기 위한 것)
			commitMsg.NodeID = node.NodeID

			LogStage("Prepare", true)
			node.Broadcast(commitMsg, "/commit")
			LogStage("Commit", false)
		}
	} else {
		//fmt.Println("Prepare is Complete")
		return nil
	}

	return nil
}

// 합의 진행
func (node *Node) GetCommit(commitMsg *VoteMsg) error {
	LogMsg(commitMsg)
	if node.CurrentState == nil {
		return nil
	}

	if node.CurrentState.CurrentStage == Prepared {
		committedMsg, err := node.CurrentState.Commit(commitMsg, node.Blockchain)
		if err != nil {
			return err
		}

		if committedMsg != nil {
			if committedMsg == nil {
				return errors.New("committed message is nil, even though the reply message is not nil")
			}

			// Save the last version of committed messages to node.
			node.CommittedMsgs = append(node.CommittedMsgs, committedMsg)

			LogStage("Commit", true)
			node.Reply(committedMsg)
			LogStage("Reply", true)
		}
	} else {
		fmt.Println("Commit is complete")
		return nil
	}

	return nil
}

// 응답 받기 - 더 돌지 않는 이유는 응답을 받고나서 아무 동작도 하지 않기 때문
func (node *Node) GetReply() error {
	// 응답 처리
	if len(node.MsgBuffer.ReqMsgs) != 0 {
		node.Flag++
		var req *RequestMsg
		//버퍼 정리
		if len(node.MsgBuffer.ReqMsgs) == 1 {
			req = node.MsgBuffer.ReqMsgs[0]
			node.MsgBuffer.ReqMsgs = make([]*RequestMsg, 0)
		} else {
			req = node.MsgBuffer.ReqMsgs[0]
			node.MsgBuffer.ReqMsgs = node.MsgBuffer.ReqMsgs[1:]
		}

		fmt.Printf("%d block reply\n", req.Height)
		node.ReplyResult(req)
	}

	return nil
}

//블록 삭제
func (node *Node) DeleteBlock(msg *ReplyMsg) {
	blen := len(node.Blockchain.Blocks) - 1
	if node.Blockchain.Blocks[blen].Height == msg.Height {
		node.Blockchain.Blocks = node.Blockchain.Blocks[:len(node.Blockchain.Blocks)-1]
	} else {
		node.Flag = 0
		fmt.Println("Not Delete. Init Flag")
	}
}

//응답
func (node *Node) ReplyResult(msg *RequestMsg) error {
	remsg := &EndMsg{
		Hash:   msg.Hash,
		Height: msg.Height,
	}
	jsonMsg, err := json.Marshal(remsg)
	if err != nil {
		return err
	}

	// Core 서버에 응답
	send(ReplyUrl, jsonMsg)

	for nodeID, url := range node.NodeAddressTable {
		if nodeID == node.View.Primary {
			continue
		}

		if nodeID == node.View.Reply {
			continue
		}

		jsonMsg, err := json.Marshal(msg)
		if err != nil {
			fmt.Println(err)
			continue
		}

		send(url+"/addblock", jsonMsg)
	}

	time.Sleep(1000)

	send(node.NodeAddressTable[node.View.Primary]+"/addblock", jsonMsg)

	return nil
}

//합의 진행 중에 쌓인 리퀘 메시지가 있다면 합의 다시 진행
func (node *Node) CheckReqMsg() {
	fmt.Printf("len : %d\n", len(node.MsgBuffer.ReqMsgs))
	if len(node.MsgBuffer.ReqMsgs) != 0 {
		fmt.Println("Get Msg From Buffer")
		req := node.MsgBuffer.ReqMsgs[0]
		fmt.Printf("req height : %d\n", req.Height)
		//버퍼 정리
		if len(node.MsgBuffer.ReqMsgs) == 1 {
			node.MsgBuffer.ReqMsgs = make([]*RequestMsg, 0)
		} else {
			node.MsgBuffer.ReqMsgs = node.MsgBuffer.ReqMsgs[1:]
		}

		node.GetReq(req)
	}
}

// 합의 생성(이미 진행중인 합의가 있다면 종료)
func (node *Node) createStateForNewConsensus() error {
	// Check if there is an ongoing consensus process.
	if node.CurrentState != nil {
		return errors.New("another consensus is ongoing")
	}

	// Get the last sequence ID
	var lastSequenceID int64
	if len(node.CommittedMsgs) == 0 {
		lastSequenceID = -1
	} else {
		lastSequenceID = node.CommittedMsgs[len(node.CommittedMsgs)-1].SequenceID
	}

	// Create a new state for this new consensus process in the Primary
	node.CurrentState = CreateState(node.View.ID, lastSequenceID)

	// 합의 시작을 위해 스테이터스 레플리카 생성(합의 스테이지 시작)
	LogStage("Create the replica status", true)

	return nil
}

//메세지 받기
func (node *Node) dispatchMsg() {
	for {
		select {
		//받은 리퀘스트 메시지를 디코드한 다음 이쪽으로 넘긴다
		case msg := <-node.MsgEntrance:
			var wg sync.WaitGroup
			wg.Add(1)
			err := node.routeMsg(msg, &sync.Mutex{}, &wg)
			if err != nil {
				fmt.Println(err)
				// TODO: send err to ErrorChannel
			}
			wg.Wait()
		case <-node.Alarm:
			err := node.routeMsgWhenAlarmed()
			if err != nil {
				fmt.Println(err)
				// TODO: send err to ErrorChannel
			}
		}
	}
}

//노드 상태 초기화
func (node *Node) InitState() {
	if node.CurrentState != nil {
		if node.Flag == 0 {
			//초기화
			fmt.Printf("초기화\n")
			node.CurrentState = nil
			node.CommittedMsgs = make([]*RequestMsg, 0)
			node.MsgBuffer.PrePrepareMsgs = make([]*PrePrepareMsg, 0)
			node.MsgBuffer.PrepareMsgs = make([]*VoteMsg, 0)
			node.MsgBuffer.CommitMsgs = make([]*VoteMsg, 0)
			node.Flag = 1
		}
	}
}

//전달받은 메세지를 메세지 타입에 맞게 처리
func (node *Node) routeMsg(msg interface{}, mutex *sync.Mutex, wg *sync.WaitGroup) []error {
	if node.CurrentState != nil {
		fmt.Printf("current State : %d\n", node.CurrentState.CurrentStage)
	}
	mutex.Lock()
	switch msg.(type) {
	case *RequestMsg:
		if node.NodeID == node.View.Reply {
			if node.Flag < msg.(*RequestMsg).Height {
				fmt.Printf("current flag : %d\n", node.Flag)
				fmt.Printf("msg height : %d\n", msg.(*RequestMsg).Height)
				// 응답할 메세지가 왔을 때
				if len(node.MsgBuffer.ReqMsgs) != 0 {
					length := len(node.MsgBuffer.ReqMsgs) - 1
					if msg.(*RequestMsg).Height > node.MsgBuffer.ReqMsgs[length].Height {
						node.MsgBuffer.ReqMsgs = append(node.MsgBuffer.ReqMsgs, msg.(*RequestMsg))
					}
				} else {
					fmt.Println("buffer add")
					node.MsgBuffer.ReqMsgs = append(node.MsgBuffer.ReqMsgs, msg.(*RequestMsg))
				}
				node.GetReply()
			}
		} else {
			if len(node.MsgBuffer.ReqMsgs) == 0 {
				//stage init
				node.InitState()
			}
			fmt.Printf("msg height : %d\n", msg.(*RequestMsg).Height)
			if node.CurrentState == nil && len(node.MsgBuffer.ReqMsgs) == 0 {
				// Copy buffered messages first.
				msgs := make([]*RequestMsg, len(node.MsgBuffer.ReqMsgs))
				//copy(msgs, node.MsgBuffer.ReqMsgs)

				// Append a newly arrived message.
				msgs = append(msgs, msg.(*RequestMsg))

				// Empty the buffer.
				//node.MsgBuffer.ReqMsgs = make([]*RequestMsg, 0)
				// Send messages.
				node.MsgDelivery <- msgs
			} else {
				if node.NodeID == node.View.Primary {
					//fmt.Printf("ReqMsg Save into Buffer\n")
					node.MsgBuffer.ReqMsgs = append(node.MsgBuffer.ReqMsgs, msg.(*RequestMsg))
					//fmt.Printf("Buffer len : %d\n", len(node.MsgBuffer.ReqMsgs))
				}
			}
			if node.NodeID != node.View.Primary {
				//fmt.Printf("SubNode\n")
				// Send messages.
				node.GetReq(msg.(*RequestMsg))
			}
		}
	case *PrePrepareMsg:
		//stage init
		node.InitState()
		if node.CurrentState == nil {
			// Copy buffered messages first.
			msgs := make([]*PrePrepareMsg, len(node.MsgBuffer.PrePrepareMsgs))
			copy(msgs, node.MsgBuffer.PrePrepareMsgs)

			// Append a newly arrived message.
			msgs = append(msgs, msg.(*PrePrepareMsg))

			// Empty the buffer.
			node.MsgBuffer.PrePrepareMsgs = make([]*PrePrepareMsg, 0)

			// Send messages.
			node.MsgDelivery <- msgs
		} else {
			node.MsgBuffer.PrePrepareMsgs = append(node.MsgBuffer.PrePrepareMsgs, msg.(*PrePrepareMsg))
		}
	case *VoteMsg:
		if msg.(*VoteMsg).MsgType == PrepareMsg {
			if node.CurrentState == nil || node.CurrentState.CurrentStage != PrePrepared {
				node.MsgBuffer.PrepareMsgs = append(node.MsgBuffer.PrepareMsgs, msg.(*VoteMsg))
			} else {
				// Copy buffered messages first.
				msgs := make([]*VoteMsg, len(node.MsgBuffer.PrepareMsgs))
				copy(msgs, node.MsgBuffer.PrepareMsgs)

				// Append a newly arrived message.
				msgs = append(msgs, msg.(*VoteMsg))

				// Empty the buffer.
				node.MsgBuffer.PrepareMsgs = make([]*VoteMsg, 0)

				// Send messages.
				node.MsgDelivery <- msgs
			}
		} else if msg.(*VoteMsg).MsgType == CommitMsg {
			if node.CurrentState == nil || node.CurrentState.CurrentStage != Prepared || node.Flag == 0 {
				node.MsgBuffer.CommitMsgs = append(node.MsgBuffer.CommitMsgs, msg.(*VoteMsg))
			} else {
				// Copy buffered messages first.
				msgs := make([]*VoteMsg, len(node.MsgBuffer.CommitMsgs))
				copy(msgs, node.MsgBuffer.CommitMsgs)

				// Append a newly arrived message.
				msgs = append(msgs, msg.(*VoteMsg))

				// Empty the buffer.
				node.MsgBuffer.CommitMsgs = make([]*VoteMsg, 0)

				// Send messages.
				node.MsgDelivery <- msgs
			}
		}
	}
	mutex.Unlock()
	wg.Done()

	return nil
}

//에러 처리
func (node *Node) routeMsgWhenAlarmed() []error {
	if node.CurrentState == nil {
		// Check ReqMsgs, send them.
		if len(node.MsgBuffer.ReqMsgs) != 0 && node.NodeID != node.View.Reply {
			fmt.Println("restart req")
			msgs := make([]*RequestMsg, len(node.MsgBuffer.ReqMsgs))
			copy(msgs, node.MsgBuffer.ReqMsgs)

			node.MsgDelivery <- msgs
		}

		// Check PrePrepareMsgs, send them.
		if len(node.MsgBuffer.PrePrepareMsgs) != 0 && node.NodeID != node.View.Reply {
			fmt.Println("restart preprepare")
			msgs := make([]*PrePrepareMsg, len(node.MsgBuffer.PrePrepareMsgs))
			copy(msgs, node.MsgBuffer.PrePrepareMsgs)

			node.MsgDelivery <- msgs
		}
	} else {
		switch node.CurrentState.CurrentStage {
		case PrePrepared:
			// Check PrepareMsgs, send them.
			if len(node.MsgBuffer.PrepareMsgs) != 0 && node.NodeID != node.View.Reply {
				fmt.Println("restart prepare")
				msgs := make([]*VoteMsg, len(node.MsgBuffer.PrepareMsgs))
				copy(msgs, node.MsgBuffer.PrepareMsgs)

				node.MsgDelivery <- msgs
			}
		case Prepared:
			// Check CommitMsgs, send them.
			if len(node.MsgBuffer.CommitMsgs) != 0 && node.NodeID != node.View.Reply {
				fmt.Println("restart commit")
				msgs := make([]*VoteMsg, len(node.MsgBuffer.CommitMsgs))
				copy(msgs, node.MsgBuffer.CommitMsgs)

				node.MsgDelivery <- msgs
			}
		}
	}

	return nil
}

//받은 메세지를 처리
func (node *Node) resolveMsg(mutex *sync.Mutex) {
	for {
		// Get buffered messages from the dispatcher.
		// 디스패쳐(메세지를 받아오는 함수)에서 넘어온 메세지를 받아서 처리한다
		msgs := <-node.MsgDelivery
		mutex.Lock()
		// 메세지 타입 확인
		switch msgs.(type) {
		// 리퀘스트 메세지였다면
		case []*RequestMsg:
			errs := node.resolveRequestMsg(msgs.([]*RequestMsg))
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
				}
				//node.alarmToDispatcher()
				// TODO: send err to ErrorChannel
			}
		case []*PrePrepareMsg:
			errs := node.resolvePrePrepareMsg(msgs.([]*PrePrepareMsg))
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
				}
				// TODO: send err to ErrorChannel
				node.alarmToDispatcher()
			}
		case []*VoteMsg:
			voteMsgs := msgs.([]*VoteMsg)
			if len(voteMsgs) == 0 {
				break
			}

			if voteMsgs[0].MsgType == PrepareMsg {
				errs := node.resolvePrepareMsg(voteMsgs)
				if len(errs) != 0 {
					for _, err := range errs {
						fmt.Println(err)
					}
					// TODO: send err to ErrorChannel
					node.alarmToDispatcher()
				}
			} else if voteMsgs[0].MsgType == CommitMsg {
				errs := node.resolveCommitMsg(voteMsgs)
				if len(errs) != 0 {
					for _, err := range errs {
						fmt.Println(err)
					}
					// TODO: send err to ErrorChannel
					node.alarmToDispatcher()
				}
			}
		}
		mutex.Unlock()
	}
}

// 특정 시간이 지나면 알람(고루프)
func (node *Node) alarmToDispatcher() {
	for {
		time.Sleep(ResolvingTimeDuration)
		node.Alarm <- true
	}
}

//리퀘스트 메세지 처리
func (node *Node) resolveRequestMsg(msgs []*RequestMsg) []error {
	errs := make([]error, 0)

	// Resolve messages
	for _, reqMsg := range msgs {
		err := node.GetReq(reqMsg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

//합의 준비 메세지 처리
func (node *Node) resolvePrePrepareMsg(msgs []*PrePrepareMsg) []error {
	errs := make([]error, 0)

	// Resolve messages
	for _, prePrepareMsg := range msgs {
		err := node.GetPrePrepare(prePrepareMsg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

//합의 대기 메세지 처리
func (node *Node) resolvePrepareMsg(msgs []*VoteMsg) []error {
	errs := make([]error, 0)

	// Resolve messages
	for _, prepareMsg := range msgs {
		err := node.GetPrepare(prepareMsg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

//합의 완료 메세지 처리
func (node *Node) resolveCommitMsg(msgs []*VoteMsg) []error {
	errs := make([]error, 0)

	// Resolve messages
	for _, commitMsg := range msgs {
		err := node.GetCommit(commitMsg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}
