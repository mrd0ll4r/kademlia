package kademlia

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValue_Store(t *testing.T) {
	vs := newValueStore()

	/*
	 * Test a simple put-get and a get for a non-existing value.
	 */
	k1 := KeyFromBytes([]byte("aaaaaaaaaaaaaaaaaaaa"))
	v1 := []byte("aaa")
	err := vs.put(k1, v1)
	require.Nil(t, err)

	ret, err := vs.get(k1)
	require.Nil(t, err)
	require.True(t, bytes.Equal(ret, v1))

	ret, err = vs.get(KeyFromBytes([]byte("bbbbbbbbbbbbbbbbbbbb")))
	require.Equal(t, err, errDNE)
}
