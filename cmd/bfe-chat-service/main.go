// Command bfe-chat-service starts the BFE Chat Service process.
// Public listeners are intentionally disabled until the approved
// BFE Auth UDS and DPoP connection boundaries are implemented.
package main

import (
	"log"
	"os"
)

func main() {
	log.SetFlags(0)
	log.Println("bfe-chat-service: startup blocked until authenticated transport is configured")
	os.Exit(1)
}
