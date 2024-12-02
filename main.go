package main

import (
	"context"
	"crypto/sha256"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	mrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/celestiaorg/merkletree"
)

const (
	url        = "http://127.0.0.1:41379/ext/bc/qVnn172mjbEzRtCqqxyWhQwJ7jNX3dTjRyNjPTLr4x8zoLj6J"
	networkID  = 1337
	chainIDStr = "qVnn172mjbEzRtCqqxyWhQwJ7jNX3dTjRyNjPTLr4x8zoLj6J"
)

type LightHeader struct {
	Height   uint64
	DataRoot []byte
	RowRoots [][]byte
	ColRoots [][]byte
}

func main() {
	log.Println("starting light client")
	wsc, err := rpc.NewWebSocketClient(url, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
	if err != nil {
		log.Fatalln(err)
	}
	chainID, _ := ids.FromString(chainIDStr)
	ctx := context.Background()
	mcli := mrpc.NewJSONRPCClient(url, networkID, chainID)
	parser, _ := mcli.Parser(ctx)
	cli := rpc.NewJSONRPCClient(url)
	lhchan := make(chan LightHeader)
	err = wsc.RegisterLightClient()
	if err != nil {
		log.Fatalln(err)
	}
	var validHeight uint64
	first := true
	go func() {
		for {
			log.Println("listening for light headers")
			hght, dataRoot, rowRoots, colRoots, err := wsc.ListenLightHeaders(ctx, parser)
			if err != nil {
				log.Fatalln(err)
			}
			if first {
				validHeight = hght
				first = false
			}
			lhchan <- LightHeader{
				Height:   hght,
				DataRoot: dataRoot,
				RowRoots: rowRoots,
				ColRoots: colRoots,
			}
		}
	}()

	go func() {
		for {
			lh := <-lhchan
			if len(lh.RowRoots) != len(lh.ColRoots) {
				log.Fatalln("invalid light header, height: ", lh.Height)
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
			log.Printf("recevied light header for block %d, rows: %d\n", lh.Height, w)
			rand.Seed(time.Now().UnixNano())
			var k int         // Counter
			var mu sync.Mutex // Mutex for safe access to k

			var wg sync.WaitGroup // WaitGroup to wait for all goroutines to complete

			for i := 0; i < s; i++ {
				wg.Add(1) // Increment the WaitGroup counter
				go func() {
					defer wg.Done() // Decrement the WaitGroup counter when goroutine finishes
					rowIDx := rand.Intn(w)
					colIDx := rand.Intn(w)
					log.Printf("fetching shares for row %d, column %d for block %d\n", rowIDx, colIDx, lh.Height)
					proofs, err := cli.GetShareRowProofs(ctx, lh.Height, uint(rowIDx), uint(colIDx))
					if err != nil {
						panic(err)
					}
					if merkletree.VerifyProof(sha256.New(), lh.RowRoots[rowIDx], proofs, uint64(colIDx), uint64(w)) {
						mu.Lock()
						k++
						mu.Unlock()
					} else {
						panic("can't verify")
					}
				}()
			}

			wg.Wait() // Wait for all sampling goroutines to complete
			if k == s {
				validHeight = lh.Height
			}
			log.Printf("Total successful samples for block %d: %d of %d. updated valid height to %d\n", lh.Height, k, s, lh.Height)
		}
	}()

	for {
		log.Printf("current valid height: %d\n", validHeight)
		time.Sleep(10 * time.Second)
	}

}
