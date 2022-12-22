package mr

import (
	"crypto/sha256"
	"errors"

	sgo "github.com/SolmateDev/solana-go"
	merkle "github.com/atomixwap/go-merkle"
)

type Tree struct {
	t merkle.Tree
}

func (tree *Tree) Root() sgo.Hash {
	return sgo.HashFromBytes(tree.t.Root())
}

func Create(hashList []sgo.Hash) (*Tree, error) {
	if len(hashList) == 0 {
		return nil, errors.New("blank list")
	}
	sh := sha256.New()
	list := make([][]byte, len(hashList))
	for i := 0; i < len(list); i++ {
		list[i] = hashList[i][:]
	}
	return &Tree{t: merkle.NewTree(sh, list...)}, nil
}
