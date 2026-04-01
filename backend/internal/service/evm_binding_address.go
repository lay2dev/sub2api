package service

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/sha3"
)

const (
	evmPurposeIndex = uint32(44)
	evmCoinType     = uint32(60)
	evmAccountIndex = uint32(0)
	evmChangeIndex  = uint32(0)
)

// DeriveEVMBindingAddress derives a deterministic EVM address for the given
// user ID using the standard BIP44 path m/44'/60'/0'/0/<userID>.
func DeriveEVMBindingAddress(mnemonic string, userID int64) (string, error) {
	mnemonic = strings.TrimSpace(mnemonic)
	if mnemonic == "" {
		return "", fmt.Errorf("binding mnemonic is empty")
	}
	if userID <= 0 {
		return "", fmt.Errorf("user id must be positive")
	}
	if userID > math.MaxUint32 {
		return "", fmt.Errorf("user id exceeds bip44 address index range")
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return "", fmt.Errorf("binding mnemonic is invalid")
	}

	seed := bip39.NewSeed(mnemonic, "")
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return "", fmt.Errorf("create master key: %w", err)
	}

	derivedKey, err := deriveChildPath(masterKey,
		evmPurposeIndex+bip32.FirstHardenedChild,
		evmCoinType+bip32.FirstHardenedChild,
		evmAccountIndex+bip32.FirstHardenedChild,
		evmChangeIndex,
		uint32(userID),
	)
	if err != nil {
		return "", fmt.Errorf("derive child path: %w", err)
	}

	privKey := secp256k1.PrivKeyFromBytes(derivedKey.Key)
	publicKeyBytes := privKey.PubKey().SerializeUncompressed()[1:]
	hasher := sha3.NewLegacyKeccak256()
	_, _ = hasher.Write(publicKeyBytes)
	sum := hasher.Sum(nil)
	return "0x" + hex.EncodeToString(sum[len(sum)-20:]), nil
}

func deriveChildPath(masterKey *bip32.Key, indices ...uint32) (*bip32.Key, error) {
	current := masterKey
	var err error
	for _, idx := range indices {
		current, err = current.NewChildKey(idx)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}
