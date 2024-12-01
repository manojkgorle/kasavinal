package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	mrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/celestiaorg/merkletree"
)

const (
	url        = "http://127.0.0.1:44637/ext/bc/sobnA86EN5P46FJ7ozF48vzkhQzyx9kMo6dh1dukBvyr9Rccm"
	networkID  = 1337
	chainIDStr = "sobnA86EN5P46FJ7ozF48vzkhQzyx9kMo6dh1dukBvyr9Rccm"
)

type LightHeader struct {
	Height   uint64
	DataRoot []byte
	RowRoots [][]byte
	ColRoots [][]byte
}

func main() {
	fmt.Println("starting wsc")
	wsc, err := rpc.NewWebSocketClient(url, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		panic(err)
	}
	chainID, _ := ids.FromString(chainIDStr)
	ctx := context.Background()
	mcli := mrpc.NewJSONRPCClient(url, networkID, chainID)
	parser, _ := mcli.Parser(ctx)
	cli := rpc.NewJSONRPCClient(url)
	lhchan := make(chan LightHeader)
	fmt.Println(wsc.RegisterLightClient())
	go func() {
		for {
			fmt.Println("listening for light headers")
			hght, dataRoot, rowRoots, colRoots, err := wsc.ListenLightHeaders(ctx, parser)
			if err != nil {
				panic(err)
			}
			fmt.Println("received light header")
			lhchan <- LightHeader{
				Height:   hght,
				DataRoot: dataRoot,
				RowRoots: rowRoots,
				ColRoots: colRoots,
			}
		}
	}()

	for {
		lh := <-lhchan
		if len(lh.RowRoots) != len(lh.ColRoots) {
			panic("invalid light header")
		}
		w := len(lh.RowRoots)
		n := w * w
		// sample 8 shares if number of shares are > 16.
		// else sample half of the shares.
		var s int
		if n > 16 {
			s = 8
		} else {
			s = n / 2
		}
		rand.Seed(time.Now().UnixNano())
		for i := 0; i < s; i++ {
			var k int
			go func() {
				rowIDx := rand.Intn(w)
				colIDx := rand.Intn(w)
				fmt.Println("fetching shares for row %d, column %d for block %d", rowIDx, colIDx, lh.Height)
				proofs, err := cli.GetShareRowProofs(ctx, lh.Height, uint(rowIDx), uint(colIDx))
				if err != nil {
					panic(err)
				}
				if merkletree.VerifyProof(sha256.New(), lh.RowRoots[rowIDx], proofs, uint64(colIDx), uint64(w)) {
					k++
				} else {
					panic("cant verify")
				}
				fmt.Println("sampled successfully: %d", k)
			}()
		}
	}
}
