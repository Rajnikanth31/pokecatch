package ai

import (
	"testing"

	"github.com/aurelia/beastbound/pkg/creatures"
	"github.com/aurelia/beastbound/pkg/types"
)

// fixedRandoms makes the AI deterministic: never triggers a "mistake".
type fixedRandoms struct{}

func (fixedRandoms) IntN(n int) int    { return 0 }
func (fixedRandoms) Chance(pct int) bool { return false }

func mv(name string, el types.Element, class types.DamageClass, power, acc int) creatures.Skill {
	return creatures.Skill{Name: name, Element: el, Class: class, Power: power, Accuracy: acc}
}

func TestAIPicksSuperEffectiveMove(t *testing.T) {
	self := View{
		Element1: types.Water, Element2: types.Water, HPFrac: 1,
		Moves: []creatures.Skill{
			mv("Tackle", types.Neutral, types.Physical, 60, 100),
			mv("Aqua Jet", types.Water, types.Special, 60, 100), // STAB + 2x vs Fire
		},
	}
	opp := View{Element1: types.Fire, Element2: types.Fire, HPFrac: 1}
	dec := ChooseAction(fixedRandoms{}, Hard, self, nil, opp)
	if dec.Switch || dec.SkillIdx != 1 {
		t.Fatalf("expected AI to pick the super-effective STAB move (idx 1), got %+v", dec)
	}
}

func TestAINeverPicksImmuneMove(t *testing.T) {
	self := View{
		Element1: types.Electric, Element2: types.Electric, HPFrac: 1,
		Moves: []creatures.Skill{
			mv("Spark", types.Electric, types.Special, 65, 100), // 0x vs Earth (immune)
			mv("Scratch", types.Neutral, types.Physical, 40, 100),
		},
	}
	opp := View{Element1: types.Earth, Element2: types.Earth, HPFrac: 1}
	dec := ChooseAction(fixedRandoms{}, Hard, self, nil, opp)
	if dec.SkillIdx != 1 {
		t.Fatalf("AI must avoid the immune move, got idx %d", dec.SkillIdx)
	}
}

func TestAISwitchesAwayFromHardCounter(t *testing.T) {
	self := View{
		Element1: types.Grass, Element2: types.Grass, HPFrac: 0.2,
		Moves:    []creatures.Skill{mv("Vine", types.Grass, types.Special, 60, 100)},
	}
	opp := View{Element1: types.Fire, Element2: types.Fire, HPFrac: 1} // Fire roasts Grass
	bench := []View{{Element1: types.Water, Element2: types.Water, HPFrac: 1}}
	dec := ChooseAction(fixedRandoms{}, Hard, self, bench, opp)
	if !dec.Switch {
		t.Fatalf("low-HP Grass into Fire should switch to the Water resist, got %+v", dec)
	}
}
