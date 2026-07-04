// Command replay re-simulates a recorded match and reports whether its stored
// outcome is authentic. This is the operator-facing side of the deterministic
// anti-cheat model: feed it a match record (as written by the battle service) and
// it re-runs the engine from the seed + intents, comparing the reproduced winner
// and state digest against what was recorded.
//
//	go run ./tools/replay --match path/to/record.json
//
// Exit code 0 = verified authentic; 2 = MISMATCH (tampered or version drift).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/services/battle/internal/persistence"
)

func main() {
	matchPath := flag.String("match", "", "path to a match record JSON")
	seedPath := flag.String("dex", "data/creatures/seed.json", "creature seed path")
	flagPath := flag.String("flagships", "data/creatures/flagships.json", "flagship overlay path")
	flag.Parse()

	if *matchPath == "" {
		fmt.Fprintln(os.Stderr, "usage: replay --match <record.json>")
		os.Exit(1)
	}

	dex, err := creatures.Load(*seedPath, *flagPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load dex:", err)
		os.Exit(1)
	}

	raw, err := os.ReadFile(*matchPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read match:", err)
		os.Exit(1)
	}
	var rec persistence.MatchRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		fmt.Fprintln(os.Stderr, "parse match:", err)
		os.Exit(1)
	}

	winner, digest, turns, err := persistence.Replay(rec, dex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "replay error:", err)
		os.Exit(1)
	}

	ok := winner == rec.Winner && digest == rec.FinalDigest
	fmt.Printf("match      : %s (%s)\n", rec.MatchID, rec.Mode)
	fmt.Printf("recorded   : winner=%d digest=%d\n", rec.Winner, rec.FinalDigest)
	fmt.Printf("re-simulated: winner=%d digest=%d turns=%d\n", winner, digest, turns)
	if ok {
		fmt.Println("VERDICT    : AUTHENTIC ✔  (outcome reproduced exactly)")
		os.Exit(0)
	}
	fmt.Println("VERDICT    : MISMATCH ✘  (tampered result or engine version drift)")
	os.Exit(2)
}
