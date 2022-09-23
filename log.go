package main

import (
	"fmt"
)

// 받은 메세지를 형식에 맞게 설정해서 출력
func LogMsg(msg interface{}) {
	switch msg.(type) {
	case *RequestMsg:
		reqMsg := msg.(*RequestMsg)
		fmt.Printf("[REQUEST] Height: %d, Timestamp: %d, Operation: AddBlock\n", reqMsg.Height, reqMsg.Timestamp)
	case *PrePrepareMsg:
		prePrepareMsg := msg.(*PrePrepareMsg)
		fmt.Printf("[PREPREPARE] Height: %d, Operation: AddBlock, SequenceID: %d\n", prePrepareMsg.RequestMsg.Height, prePrepareMsg.SequenceID)
	case *VoteMsg:
		voteMsg := msg.(*VoteMsg)
		if voteMsg.MsgType == PrepareMsg {
			fmt.Printf("[PREPARE] NodeID: %s\n", voteMsg.NodeID)
		} else if voteMsg.MsgType == CommitMsg {
			fmt.Printf("[COMMIT] NodeID: %s\n", voteMsg.NodeID)
		}
	}
}

func LogStage(stage string, isDone bool) {
	if isDone {
		fmt.Printf("[STAGE-DONE] %s\n", stage)
	} else {
		fmt.Printf("[STAGE-BEGIN] %s\n", stage)
	}
}
