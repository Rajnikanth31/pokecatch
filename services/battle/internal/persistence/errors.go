package persistence

import "fmt"

// errUnknownSpecies is returned when a recorded team references a dex id the
// current Dex doesn't know — usually a sign of a data/version mismatch, which is
// itself useful anti-cheat/ops signal.
func errUnknownSpecies(dexID int) error {
	return fmt.Errorf("persistence: replay references unknown species dex_id=%d", dexID)
}
