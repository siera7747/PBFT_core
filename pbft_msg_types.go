package main

type RequestMsg struct {
	Hash          []byte `json:"Hash"`          //32바이트짜리 (ID)
	PervBlockHash []byte `json:"PrevBlockHash"` //이전 해시 값
	Timestamp     int64  `json:"Timestamp"`     //시간. 여기부터 가져와서 해시 만들기
	Pow           []byte `json:"Pow"`           //pow 값
	Nonce         int    `json:"Nonce"`
	Bits          int64  `json:"Bits"`   //타겟
	TxId          []byte `json:"TxID"`   //트랜잭션Id
	Height        int    `json:"Height"` //블록순서
	SequenceID    int64  `json:"sequenceID"`
}

type ReplyMsg struct {
	Hash   []byte `json:"Hash"`   //32바이트짜리 (ID)
	Height int    `json:"Height"` //블록순서
	NodeId string `json:"NodeID"` //노드
}

type PrePrepareMsg struct {
	ViewID     int64       `json:"viewID"`
	SequenceID int64       `json:"sequenceID"`
	Digest     string      `json:"digest"`
	RequestMsg *RequestMsg `json:"requestMsg"`
}

type VoteMsg struct {
	ViewID     int64  `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`
	Digest     string `json:"digest"`
	NodeID     string `json:"nodeID"`
	MsgType    `json:"msgType"`
}

type EndMsg struct {
	Hash   []byte `json:"Hash"`   //32바이트짜리 (ID)
	Height int    `json:"Height"` //블록순서
}

type MsgType int

const (
	PrepareMsg MsgType = iota
	CommitMsg
)
