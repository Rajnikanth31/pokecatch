// This tool moved to services/battle/cmd/replay so it can import the battle
// service's internal/persistence package (Go forbids importing another tree's
// internal/ packages from here). This stub remains only to point you there.
//
//	go run ./services/battle/cmd/replay --match record.json
package main

import "fmt"

func main() {
	fmt.Println("moved: use  go run ./services/battle/cmd/replay --match <record.json>")
}
