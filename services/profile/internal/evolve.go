package profile

import (
	"context"
	"errors"

	"github.com/aurelia/beastbound/pkg/creatures"
)

var (
	ErrCannotEvolve   = errors.New("profile: species has no evolution")
	ErrLevelTooLow    = errors.New("profile: creature level below evolution requirement")
)

// Evolve transforms a caller's creature into its next form. This is a classic
// place where trusting the client would be a mistake — the server re-checks the
// evolution rule against the immutable Dex (level gate, target species) and only
// then mutates the instance. The client just requests; the server decides.
//
// Returns the updated creature so the client can refresh its view.
func (c *Collection) Evolve(ctx context.Context, ownerID, creatureID string) (OwnedCreature, error) {
	oc, err := c.repo.GetCreature(ctx, creatureID)
	if err != nil {
		return OwnedCreature{}, err
	}
	if oc.OwnerID != ownerID {
		return OwnedCreature{}, ErrNotOwner
	}
	sp, ok := c.dex.Species[oc.Instance.DexID]
	if !ok {
		return OwnedCreature{}, ErrCannotEvolve
	}
	if sp.EvolvesToID == 0 {
		return OwnedCreature{}, ErrCannotEvolve // already final form
	}
	if oc.Instance.Level < sp.EvolveLevel {
		return OwnedCreature{}, ErrLevelTooLow
	}
	target, ok := c.dex.Species[sp.EvolvesToID]
	if !ok {
		return OwnedCreature{}, ErrCannotEvolve
	}

	// Apply the evolution: change species, keep level/XP/IVs/EVs/nickname. The
	// ability may be re-rolled to the evolved form's default if the current one
	// isn't in its pool (kept simple here: retain if compatible).
	oc.Instance.DexID = target.DexID
	oc.Instance.Ability = pickAbility(target, oc.Instance.Ability)

	if err := c.repo.UpdateCreature(ctx, oc); err != nil {
		return OwnedCreature{}, err
	}
	return oc, nil
}

// pickAbility keeps the current ability if the evolved species can have it,
// otherwise falls back to the species' first listed ability.
func pickAbility(sp *creatures.Species, current creatures.PassiveAbility) creatures.PassiveAbility {
	for _, a := range sp.Abilities {
		if a == current {
			return current
		}
	}
	if len(sp.Abilities) > 0 {
		return sp.Abilities[0]
	}
	return current
}
