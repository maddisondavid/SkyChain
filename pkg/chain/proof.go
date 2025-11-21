package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrTxNotFound indicates that a transaction hash is not present in the block being inspected.
var ErrTxNotFound = errors.New("transaction not found in block")

// ErrInvalidProof indicates that the provided proof could not be validated against the advertised Merkle root.
var ErrInvalidProof = errors.New("invalid merkle proof")

// ProofNode represents one hop in a Merkle branch, capturing the sibling hash and its relative position.
type ProofNode struct {
	Position string `json:"position"`
	Hash     string `json:"hash"`
}

// MerkleProof packages the data required for a client to verify that a transaction was included in a block.
type MerkleProof struct {
	TxID        string      `json:"txid"`
	BlockIndex  int         `json:"block_index"`
	BlockHash   string      `json:"block_hash"`
	MerkleRoot  string      `json:"merkle_root"`
	LeafHash    string      `json:"leaf_hash"`
	LeafIndex   int         `json:"leaf_index"`
	Branch      []ProofNode `json:"branch"`
	Validator   string      `json:"validator"`
	Timestamp   time.Time   `json:"timestamp"`
	TotalLeaves int         `json:"total_leaves"`
}

// EventHash returns the hex-encoded SHA256 hash of the JSON-encoded event payload.
func EventHash(evt Event) (string, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return "", fmt.Errorf("marshal event: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// BuildProofForBlock creates a Merkle proof for the provided transaction hash within the supplied block.
// It returns ErrTxNotFound when the transaction is not present.
func BuildProofForBlock(block Block, txid string) (MerkleProof, error) {
	if txid == "" {
		return MerkleProof{}, errors.New("txid is required")
	}

	leaves := make([][]byte, len(block.Events))
	targetIndex := -1
	for i, evt := range block.Events {
		hash, err := EventHash(evt)
		if err != nil {
			return MerkleProof{}, err
		}
		hashBytes, err := hex.DecodeString(hash)
		if err != nil {
			return MerkleProof{}, err
		}
		leaves[i] = hashBytes
		if hash == txid {
			targetIndex = i
		}
	}

	if targetIndex == -1 {
		return MerkleProof{}, ErrTxNotFound
	}

	branch := buildProofBranch(leaves, targetIndex)
	proof := MerkleProof{
		TxID:        txid,
		BlockIndex:  block.Index,
		BlockHash:   block.Hash,
		MerkleRoot:  block.MerkleRoot,
		LeafHash:    txid,
		LeafIndex:   targetIndex,
		Branch:      branch,
		Validator:   block.Validator,
		Timestamp:   block.Timestamp,
		TotalLeaves: len(leaves),
	}

	return proof, nil
}

// VerifyProof recomputes the Merkle root from the provided branch and leaf hash, returning ErrInvalidProof on mismatch.
func VerifyProof(proof MerkleProof) error {
	if proof.LeafHash == "" {
		return ErrInvalidProof
	}
	current, err := hex.DecodeString(proof.LeafHash)
	if err != nil {
		return ErrInvalidProof
	}

	for _, hop := range proof.Branch {
		sibling, err := hex.DecodeString(hop.Hash)
		if err != nil {
			return ErrInvalidProof
		}
		var combined []byte
		switch hop.Position {
		case "left":
			combined = append(append(make([]byte, 0, len(sibling)+len(current)), sibling...), current...)
		case "right":
			combined = append(append(make([]byte, 0, len(sibling)+len(current)), current...), sibling...)
		default:
			return ErrInvalidProof
		}
		sum := sha256.Sum256(combined)
		current = sum[:]
	}

	root := hex.EncodeToString(current)
	if root != proof.MerkleRoot {
		return ErrInvalidProof
	}
	return nil
}

func buildProofBranch(level [][]byte, idx int) []ProofNode {
	branch := make([]ProofNode, 0)
	for len(level) > 1 {
		var siblingIdx int
		position := "right"
		if idx%2 == 0 { // left leaf
			siblingIdx = idx + 1
			if siblingIdx >= len(level) {
				siblingIdx = idx
			}
		} else {
			siblingIdx = idx - 1
			position = "left"
		}
		branch = append(branch, ProofNode{Position: position, Hash: hex.EncodeToString(level[siblingIdx])})

		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			right := left
			if i+1 < len(level) {
				right = level[i+1]
			}
			combined := append(append(make([]byte, 0, len(left)+len(right)), left...), right...)
			sum := sha256.Sum256(combined)
			next = append(next, sum[:])
		}
		level = next
		idx = idx / 2
	}
	return branch
}
