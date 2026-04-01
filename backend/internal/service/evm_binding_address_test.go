//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testBindingMnemonic = "test test test test test test test test test test test junk"

func TestDeriveEVMBindingAddress_IsStableForSameUser(t *testing.T) {
	first, err := DeriveEVMBindingAddress(testBindingMnemonic, 7)
	require.NoError(t, err)

	second, err := DeriveEVMBindingAddress(testBindingMnemonic, 7)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Len(t, first, 42)
	require.Regexp(t, "^0x[0-9a-f]{40}$", first)
}

func TestDeriveEVMBindingAddress_ChangesAcrossUsers(t *testing.T) {
	first, err := DeriveEVMBindingAddress(testBindingMnemonic, 7)
	require.NoError(t, err)

	second, err := DeriveEVMBindingAddress(testBindingMnemonic, 8)
	require.NoError(t, err)

	require.NotEqual(t, first, second)
}

func TestDeriveEVMBindingAddress_RejectsUserIDAboveUint32(t *testing.T) {
	_, err := DeriveEVMBindingAddress(testBindingMnemonic, 1<<32)
	require.Error(t, err)
	require.ErrorContains(t, err, "address index range")
}
