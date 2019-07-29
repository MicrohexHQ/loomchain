// +build !evm

package auth

import (
	"fmt"

	"github.com/loomnetwork/go-loom/common/evmcompat"
)

func VerifySolidity66Byte(_ SignedTx, _ []evmcompat.SignatureType) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func verifyTron(_ SignedTx, _ []evmcompat.SignatureType) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func VerifyEthereumTransacton(_ SignedTx) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
