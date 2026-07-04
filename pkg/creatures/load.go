package creatures

import (
	"encoding/json"
	"fmt"
	"os"
)

// seedFile is the on-disk shape produced by tools/creaturegen.
type seedFile struct {
	Version int        `json:"version"`
	Species []*Species `json:"species"`
}

// flagshipFile additionally carries hand-authored skills.
type flagshipFile struct {
	Skills  []*Skill   `json:"skills"`
	Species []*Species `json:"species"`
}

// Dex is the loaded, validated catalog: species keyed by DexID and skills keyed
// by id. It is built once at boot and treated as immutable thereafter.
type Dex struct {
	Species map[int]*Species
	Skills  map[string]*Skill
}

// Load reads the generated seed and the flagship overlay, merging the latter on
// top by DexID. It validates referential integrity (evolutions and learnset
// skill ids must resolve) and returns an error rather than booting a service
// with a corrupt catalog.
func Load(seedPath, flagshipPath string) (*Dex, error) {
	var sf seedFile
	if err := readJSON(seedPath, &sf); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	var ff flagshipFile
	if err := readJSON(flagshipPath, &ff); err != nil {
		return nil, fmt.Errorf("flagships: %w", err)
	}

	d := &Dex{Species: make(map[int]*Species, len(sf.Species)), Skills: make(map[string]*Skill)}
	for _, s := range sf.Species {
		d.Species[s.DexID] = s
	}
	for _, s := range ff.Species { // overlay wins
		d.Species[s.DexID] = s
	}
	for _, sk := range ff.Skills {
		d.Skills[sk.ID] = sk
	}

	if err := d.validate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dex) validate() error {
	for id, sp := range d.Species {
		if sp.EvolvesToID != 0 {
			if _, ok := d.Species[sp.EvolvesToID]; !ok {
				return fmt.Errorf("dex %d evolves to missing species %d", id, sp.EvolvesToID)
			}
		}
		for _, le := range sp.Learnset {
			if _, ok := d.Skills[le.SkillID]; !ok {
				return fmt.Errorf("dex %d learnset references missing skill %q", id, le.SkillID)
			}
		}
	}
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
