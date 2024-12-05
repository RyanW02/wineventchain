package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"os"
)

var (
	eventIdHex   = flag.String("event_id", "", "Event ID")
	txHashHex    = flag.String("tx_hash", "", "Transaction Hash")
	eventDataStr = flag.String("event_data", "[]", "Event Data")
	principal    = flag.String("principal", "", "Principal")
	privateKey   = flag.String("priv_key", "", "Private Key (Base64 Encoded)")
)

func main() {
	flag.Parse()

	if *eventIdHex == "" || *txHashHex == "" || *principal == "" {
		flag.Usage()
		os.Exit(1)
	}

	eventId, err := hex.DecodeString(*eventIdHex)
	if err != nil {
		panic(err)
	}

	txHash, err := hex.DecodeString(*txHashHex)
	if err != nil {
		panic(err)
	}

	var eventData events.EventData
	if err := json.Unmarshal([]byte(*eventDataStr), &eventData); err != nil {
		panic(err)
	}

	privKeyDecoded, err := base64.StdEncoding.DecodeString(*privateKey)
	if err != nil {
		panic(err)
	}

	req := types.SubmitRequest{
		EventId:   eventId,
		TxHash:    txHash,
		EventData: eventData,
		Principal: *principal,
		Signature: hex.EncodeToString(ed25519.Sign(privKeyDecoded, eventData.Hash())),
	}

	marshalled, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(marshalled))
}
