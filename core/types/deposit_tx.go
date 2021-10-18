// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type DepositTx struct {
	ChainID *big.Int
	To      *common.Address `rlp:"nil"` // nil means contract creation
	// minted on L2, locked on L1
	Value *big.Int
	Data  []byte
	From  *common.Address
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *DepositTx) copy() TxData {
	cpy := &DepositTx{
		To:      tx.To, // TODO: copy pointed-to address
		Data:    common.CopyBytes(tx.Data),
		Value:   new(big.Int),
		ChainID: new(big.Int),
	}
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.ChainID != nil {
		cpy.ChainID.Set(tx.ChainID)
	}
	return cpy
}

// accessors for innerTx.
func (tx *DepositTx) txType() byte           { return DepositTxType }
func (tx *DepositTx) chainID() *big.Int      { return tx.ChainID }
func (tx *DepositTx) protected() bool        { return true }
func (tx *DepositTx) accessList() AccessList { return nil }
func (tx *DepositTx) data() []byte           { return tx.Data }
func (tx *DepositTx) gas() uint64            { return 100_000_000 } // 100 million (for free, but no refunds)
func (tx *DepositTx) gasFeeCap() *big.Int    { return new(big.Int) }
func (tx *DepositTx) gasTipCap() *big.Int    { return new(big.Int) }
func (tx *DepositTx) gasPrice() *big.Int     { return new(big.Int) }
func (tx *DepositTx) value() *big.Int        { return tx.Value }
func (tx *DepositTx) nonce() uint64          { return 0xffff_ffff_ffff_ffff }
func (tx *DepositTx) to() *common.Address    { return tx.To }

func (tx *DepositTx) rawSignatureValues() (v, r, s *big.Int) {
	panic("deposit tx does not have a signature")
}

func (tx *DepositTx) setSignatureValues(chainID, v, r, s *big.Int) {
	panic("deposit tx does not have a signature")
}
