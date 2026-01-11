package api //nolint:revive // package name is intentional

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	closed atomic.Int64
}

func (f *fakeClient) Close() error {
	f.closed.Add(1)
	return nil
}

func TestClientSwapUsesLatestClient(t *testing.T) {
	first := &fakeClient{}
	swapper := newClientSwap[*fakeClient](first)

	got, release := swapper.acquire()
	require.Same(t, first, got)
	release()

	next := &fakeClient{}
	swapper.swap(next)

	got, release = swapper.acquire()
	require.Same(t, next, got)
	release()
}

func TestClientSwapDefersCloseUntilRelease(t *testing.T) {
	first := &fakeClient{}
	swapper := newClientSwap[*fakeClient](first)

	got, release := swapper.acquire()
	require.Same(t, first, got)

	next := &fakeClient{}
	swapper.swap(next)

	require.Equal(t, int64(0), first.closed.Load())

	release()

	require.Equal(t, int64(1), first.closed.Load())
}

func TestClientSwapClosesIdleClientOnSwap(t *testing.T) {
	first := &fakeClient{}
	swapper := newClientSwap[*fakeClient](first)

	next := &fakeClient{}
	swapper.swap(next)

	require.Equal(t, int64(1), first.closed.Load())
}
