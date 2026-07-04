package engine

import "github.com/aurelia/beastbound/pkg/creatures"

// Registry holds the static skill and species tables loaded once at boot from
// the JSON seed. The battle engine reads from it but never mutates it, so a
// single shared pointer is safe across all concurrent battles (read-only).
type Registry struct {
	Skills  map[string]*creatures.Skill
	Species map[int]*creatures.Species
}

// activeRegistry is process-global. Set once during service start via SetRegistry.
var activeRegistry *Registry

// SetRegistry installs the immutable data tables. Must be called before any
// Battle is created.
func SetRegistry(r *Registry) { activeRegistry = r }

// skillFor resolves the Skill in a battler's slot via the active registry.
func skillFor(b *Battler, slot int) *creatures.Skill {
	if slot < 0 || slot >= len(b.Inst.SkillIDs) || activeRegistry == nil {
		return nil
	}
	id := b.Inst.SkillIDs[slot]
	if id == "" {
		return nil
	}
	return activeRegistry.Skills[id]
}
