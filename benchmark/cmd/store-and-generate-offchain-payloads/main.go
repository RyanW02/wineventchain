package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/blockchain/helpers"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"github.com/cometbft/cometbft/rpc/client/http"
	"github.com/google/uuid"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
	"net/url"
	"os"
	"sync"
)

var (
	eventFile     = flag.String("event_file", "", "XML event file path")
	tendermintUri = flag.String("tendermint_uri", "http://localhost:26657", "Tendermint URI")
	principal     = flag.String("principal", "", "Principal")
	privateKeyRaw = flag.String("priv_key", "", "Private key (Base64 encoded)")
	n             = flag.Int("n", 0, "Number of events to generate")
	workers       = flag.Int("workers", 50, "Number of workers to use for submitting events")
	outFile       = flag.String("out_file", "out.json", "Output file")
)

func main() {
	flag.Parse()

	if *eventFile == "" || *principal == "" || *privateKeyRaw == "" || *n == 0 {
		flag.Usage()
		os.Exit(1)
	}

	xmlRaw, err := os.ReadFile(*eventFile)
	if err != nil {
		panic(err)
	}

	var event events.EventWithData
	if err := xml.Unmarshal(xmlRaw, &event); err != nil {
		panic(err)
	}

	privateKey, err := base64.StdEncoding.DecodeString(*privateKeyRaw)
	if err != nil {
		panic(err)
	}

	client, err := buildBlockchainClient()
	if err != nil {
		panic(err)
	}

	ch := make(chan []byte, *n)
	for i := 0; i < *n; i++ {
		event.Event.System.EventRecordId = i

		payload, err := rpc.NewBuilder().
			App(events.AppName).
			Data(events.RequestTypeCreate, events.CreateRequest{
				Event: events.ScrubbedEvent{
					OffChainHash: hex.EncodeToString(event.EventData.Hash()),
					Event:        event.Event,
				},
				Nonce: uuid.New(),
			}).
			Signed(identity.Principal(*principal), privateKey).
			Marshal()

		if err != nil {
			panic(err)
		}

		ch <- payload
	}

	close(ch)

	// Start workers
	group, _ := errgroup.WithContext(context.Background())
	counter := atomic.NewInt32(0)

	lock := sync.Mutex{}
	requests := make([]types.SubmitRequest, 0, *n)

	for i := 0; i < *workers; i++ {
		group.Go(func() error {
			for payload := range ch {
				res, err := helpers.BroadcastAndPollDefault(context.Background(), client, payload)
				if err != nil {
					return err
				}

				var data events.CreateResponse
				if err := json.Unmarshal(res.TxResult.Data, &data); err != nil {
					return err
				}

				req := types.SubmitRequest{
					EventId:   data.Metadata.EventId,
					TxHash:    res.Hash.Bytes(),
					EventData: event.EventData,
					Principal: *principal,
					Signature: hex.EncodeToString(ed25519.Sign(privateKey, event.EventData.Hash())),
				}

				lock.Lock()
				requests = append(requests, req)
				lock.Unlock()

				count := counter.Inc()
				if count%100 == 0 {
					fmt.Printf("Done %d/%d\n", count, *n)
				}
			}

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		panic(err)
	}

	out, err := json.Marshal(requests)
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(*outFile, out, 0644); err != nil {
		panic(err)
	}

	fmt.Println("Done!")
}

func buildBlockchainClient() (*http.HTTP, error) {
	uri, err := url.Parse(*tendermintUri)
	if err != nil {
		return nil, err
	}

	uri.Path = "/"
	remote := uri.String()

	uri.Path = "/websocket"
	ws := uri.String()

	return http.New(remote, ws)
}
