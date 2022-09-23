package main

import (
	"bytes"
	"fmt"

	"github.com/google/go-cmp/cmp"
)

type Blockchain struct {
	Blocks []*Block
}

//블록체인 구조체 안에 존재하는 함수.
//블록체인에 블록을 추가한다.
func (blockchain *Blockchain) AddBlock(block *Block) {
	//블록체인에 블록 추가
	blockchain.Blocks = append(blockchain.Blocks, block)
}

// 블록 찾기(해시 데이터(BlockID) 이용)
func (bc *Blockchain) FindB(e []byte) *Block {
	for _, v := range bc.Blocks {
		if v.EqHash(e) {
			return v
		}
	}
	return nil
}

//가장 마지막으로 저장한 블록의 해시값과 같은지 확인하기
func (bc *Blockchain) CheckPrevHash(hash []byte) bool {
	prevB := bc.Blocks[len(bc.Blocks)-1].Hash[:]
	// fmt.Printf("prevB : %+v\n", prevB)
	// fmt.Printf("CurrentB : %+v\n", hash)
	return bytes.Equal(prevB, hash)
}

//블록 데이터 확인하기
func (bc *Blockchain) ReadData(od *Blockchain) bool {
	for i, v := range bc.Blocks {
		fmt.Printf("%d번째 블록\n", i)
		fmt.Printf("%+v\n", v)
		fmt.Println()

		// if !bytes.Equal(v.Hash, od.Blocks[i].Hash) {
		// 	return false
		// }
		if !cmp.Equal(v, od.Blocks[i]) {
			return false
		}
	}
	return true
}

func NewBlockchain() *Blockchain {
	return &Blockchain{[]*Block{}}
}
