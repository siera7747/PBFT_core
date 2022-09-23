package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type State struct {
	ViewID         int64
	MsgLogs        *MsgLogs
	LastSequenceID int64
	CurrentStage   Stage
}

type MsgLogs struct {
	ReqMsg      *RequestMsg
	PrepareMsgs map[string]*VoteMsg
	CommitMsgs  map[string]*VoteMsg
}

type Stage int

const (
	Idle        Stage = iota // Node is created successfully, but the consensus process is not started yet.
	PrePrepared              // The ReqMsgs is processed successfully. The node is ready to head to the Prepare stage.
	Prepared                 // Same with `prepared` stage explained in the original paper.
	Committed                // Same with `committed-local` stage explained in the original paper.
)

// f: # of Byzantine faulty node
// f = (n­1) / 3
// n = 4, in this case.
const f = 1

// lastSequenceID will be -1 if there is no last sequence ID.
// 상태 생성
func CreateState(viewID int64, lastSequenceID int64) *State {
	return &State{
		ViewID: viewID,
		MsgLogs: &MsgLogs{
			ReqMsg:      nil,
			PrepareMsgs: make(map[string]*VoteMsg),
			CommitMsgs:  make(map[string]*VoteMsg),
		},
		LastSequenceID: lastSequenceID,
		CurrentStage:   Idle,
	}
}

//합의 시작(리퀘스트 메시지 처리)
func (state *State) StartConsensus(request *RequestMsg) (*PrePrepareMsg, error) {
	// `sequenceID` will be the index of this message.
	sequenceID := time.Now().UnixNano()

	// Find the unique and largest number for the sequence ID
	if state.LastSequenceID != -1 {
		for state.LastSequenceID >= sequenceID {
			sequenceID += 1
		}
	}

	// Assign a new sequence ID to the request message object.
	request.SequenceID = sequenceID

	// Save ReqMsgs to its logs.
	state.MsgLogs.ReqMsg = request

	// Get the digest of the request message
	digest, err := digest(request)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Change the stage to pre-prepared.
	state.CurrentStage = PrePrepared

	return &PrePrepareMsg{
		ViewID:     state.ViewID,
		SequenceID: sequenceID,
		Digest:     digest,
		RequestMsg: request,
	}, nil
}

//합의 준비
func (state *State) PrePrepare(prePrepareMsg *PrePrepareMsg, bc *Blockchain) (*VoteMsg, error) {
	// Get ReqMsgs and save it to its logs like the primary.
	state.MsgLogs.ReqMsg = prePrepareMsg.RequestMsg

	// Verify if v, n(a.k.a. sequenceID), d are correct.
	if !state.verifyMsg(prePrepareMsg.ViewID, prePrepareMsg.SequenceID, prePrepareMsg.Digest, bc) {
		return nil, errors.New("pre-prepare message is corrupted")
	}

	// Change the stage to pre-prepared.
	state.CurrentStage = PrePrepared

	return &VoteMsg{
		ViewID:     state.ViewID,
		SequenceID: prePrepareMsg.SequenceID,
		Digest:     prePrepareMsg.Digest,
		MsgType:    PrepareMsg,
	}, nil
}

//합의 대기
func (state *State) Prepare(prepareMsg *VoteMsg, bc *Blockchain) (*VoteMsg, error) {
	if !state.verifyMsg(prepareMsg.ViewID, prepareMsg.SequenceID, prepareMsg.Digest, bc) {
		return nil, errors.New("prepare message is corrupted")
	}

	// Append msg to its logs
	state.MsgLogs.PrepareMsgs[prepareMsg.NodeID] = prepareMsg

	// Print current voting status
	fmt.Printf("[Prepare-Vote]: %d\n", len(state.MsgLogs.PrepareMsgs))

	if state.prepared() {
		// Change the stage to prepared.
		state.CurrentStage = Prepared

		return &VoteMsg{
			ViewID:     state.ViewID,
			SequenceID: prepareMsg.SequenceID,
			Digest:     prepareMsg.Digest,
			MsgType:    CommitMsg,
		}, nil
	}

	return nil, nil
}

//합의 완료
func (state *State) Commit(commitMsg *VoteMsg, bc *Blockchain) (*RequestMsg, error) {
	if !state.verifyMsg(commitMsg.ViewID, commitMsg.SequenceID, commitMsg.Digest, bc) {
		return nil, errors.New("commit message is corrupted")
	}

	// Append msg to its logs
	state.MsgLogs.CommitMsgs[commitMsg.NodeID] = commitMsg

	// Print current voting status
	fmt.Printf("[Commit-Vote]: %d\n", len(state.MsgLogs.CommitMsgs))

	if state.committed() {
		fmt.Printf("commit complete\n")
		// Change the stage to prepared.
		state.CurrentStage = Committed

		return state.MsgLogs.ReqMsg, nil
	}

	return nil, nil
}

//메세지 검사
func (state *State) verifyMsg(viewID int64, sequenceID int64, digestGot string, bc *Blockchain) bool {
	// Wrong view. That is, wrong configurations of peers to start the
	if state.ViewID != viewID {
		fmt.Println("ViewID is corrupted")
		return false
	}

	// Check if the Primary sent fault sequence number. => Faulty primary.
	// TODO: adopt upper/lower bound check.
	if state.LastSequenceID != -1 {
		if state.LastSequenceID >= sequenceID {
			fmt.Println("sequenceID is corrupted")
			return false
		}
	}

	digest, err := digest(state.MsgLogs.ReqMsg)
	if err != nil {
		fmt.Println(err)
		fmt.Println("digest failed")
		return false
	}

	// Check digest.
	if digestGot != digest {
		fmt.Println("digest is corrupted")
		return false
	}

	//블록 검증(이전 해시값과 같은지 확인. 처음으로 추가하는 블록이라면 확인하지 않음)
	if len(bc.Blocks) != 0 {
		if !bc.CheckPrevHash(state.MsgLogs.ReqMsg.PervBlockHash) {
			fmt.Printf("Block verify failed.\n")
			return false
		}
	} else {
		fmt.Printf("GenesisBlock\n")
	}

	return true
}

//합의 대기 종료 여부 체크
func (state *State) prepared() bool {
	if state.MsgLogs.ReqMsg == nil {
		return false
	}

	if len(state.MsgLogs.PrepareMsgs) < 2*f {
		return false
	}

	return true
}

//합의 완료 여부 체크
func (state *State) committed() bool {
	if !state.prepared() {
		return false
	}

	if len(state.MsgLogs.CommitMsgs) < 2*f {
		return false
	}

	return true
}

func digest(object interface{}) (string, error) {
	msg, err := json.Marshal(object)

	if err != nil {
		return "", err
	}

	return Hash(msg), nil
}
