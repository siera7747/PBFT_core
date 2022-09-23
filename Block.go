package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strconv"
	"time"
)

type Block struct {
	Hash          []byte `json:"Hash"` //32바이트짜리 (ID)
	PervBlockHash []byte `json:"Prev"` //이전 해시 값
	Timestamp     int64  `json:"Time"` //시간. 여기부터 가져와서 해시 만들기
	Pow           []byte `json:"Pow"`  //pow 값
	Nonce         int    `json:"Nonce"`
	Bits          int64  `json:"Bits"`   //타겟
	TxId          []byte `json:"TraxId"` //트랜잭션Id
	Height        int    `json:"Height"` //블록순서
	//signature     []byte  //개인키로 서명(Hash)
}

func (b *Block) print() {
	// json 출력 방식
	fmt.Printf("Block %d\n", b.Height)
	fmt.Printf("%+v\n", b)
	fmt.Println()
}

// 해시값 비교
func (b *Block) EqHash(e []byte) bool {
	return bytes.Equal(b.Hash, e)
}

// 데이터 비교
func (b *Block) EqData(e []byte) bool {
	return bytes.Equal(b.TxId, e)
}

// 제네시스블록인지 확인
func (b *Block) IsGBc() bool {
	return bytes.Equal(b.PervBlockHash, make([]byte, len(b.PervBlockHash)))
}

func (block *Block) getHash() {

	timestamp := strconv.FormatInt(block.Timestamp, 10)
	timeBytes := []byte(timestamp)

	nonstr := string(block.Nonce)
	nonBytes := []byte(nonstr)

	bitBytes := []byte(strconv.Itoa(int(block.Bits)))

	heightstr := string(block.Height)
	hBytes := []byte(heightstr)

	//---------------------------------
	// blockBytes 값 채우기(블록 구조체 내용물 채우기)
	// 블록 구성요소들을 하나로 모아서 해시로 만듬
	blockBytes := bytes.Join([][]byte{timeBytes, block.Pow, nonBytes, bitBytes, block.TxId, hBytes}, []byte{})
	//---------------------------------
	hash := sha256.Sum256(blockBytes)
	block.Hash = hash[:]
}

func NewBlock(txId []byte, pervBlockHash []byte, h int) *Block {

	block := &Block{}
	//----------------------------
	//  block element 값 채우기
	block.PervBlockHash = pervBlockHash
	block.Timestamp = time.Now().UTC().Unix()
	block.TxId = txId

	block.Height = h + 1

	rad, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		panic(err)
	}
	bt, _ := strconv.Atoi(rad.String())
	block.Bits = int64(bt)

	//----------------------------
	pow := newProofOfWork(block)
	nonce, hash := pow.Run()
	block.Pow = hash[:]
	block.Nonce = nonce

	block.getHash()

	return block
}

func NewGenesisBlock() *Block {
	return NewBlock([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, -1)
}

func Printbc(b *Block) {
	b.print()
}
