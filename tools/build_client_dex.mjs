// Builds client-godot/data/dex.json for the phone game: all 300 generated
// creatures (proper rarity spread + catch rates) each given a type-appropriate
// moveset, PLUS the 20 hand-authored flagships with their bespoke skills. This
// gives the game a real catchable world where commons are beatable and legendaries
// are rare — verified by the balance sim.
import { readFileSync, writeFileSync } from 'node:fs';

const seed = JSON.parse(readFileSync(new URL('../data/creatures/seed.json', import.meta.url)));
const flag = JSON.parse(readFileSync(new URL('../data/creatures/flagships.json', import.meta.url)));

const ELNAME = ["Strike","Flame","Aqua","Vine","Spark","Tremor","Gust","Frost","Sludge","Psywave","Shadow","Ironclad"];
// One generic move per element. Element 0 = Neutral "Strike" (physical); the rest
// are special elemental attacks. Balanced modest power so battles last a few turns.
const genericSkills = ELNAME.map((name, el) => ({
  id: "g_" + el,
  name: name,
  element: el,
  class: el === 0 ? 0 : 1,
  power: el === 0 ? 45 : 55,
  accuracy: 100,
  pp: 20,
}));

// Merge skills: generic + flagship.
const skills = [...genericSkills, ...flag.skills];

// Generic learnset: Strike + STAB for each type the creature has.
function genericLearnset(sp) {
  const ls = [{ level: 1, skill_id: "g_0" }];
  ls.push({ level: 1, skill_id: "g_" + sp.element1 });
  if (sp.element2 !== sp.element1) ls.push({ level: 1, skill_id: "g_" + sp.element2 });
  return ls;
}

// Start from all 300 seed species with generated movesets.
const byId = {};
for (const s of seed.species) {
  byId[s.dex_id] = { ...s, learnset: genericLearnset(s) };
}
// Overlay the 20 flagships (their own skills + learnsets + lore).
for (const s of flag.species) byId[s.dex_id] = s;

// Starters are your bonded partner and should punch above common creatures — buff
// the three starter families ~20% so early battles feel winnable (balance-tested).
const STARTER_FAMILIES = [1, 2, 3, 4, 5, 6, 7, 8, 9];
for (const id of STARTER_FAMILIES) {
  const b = byId[id].base;
  for (const k of Object.keys(b)) b[k] = Math.round(b[k] * 1.2);
}

const out = {
  version: 2,
  note: "Client game dex: 300 generated creatures with type movesets + 20 flagships. Elements 0-11.",
  skills,
  species: Object.values(byId).sort((a, b) => a.dex_id - b.dex_id),
};
const path = new URL('../client-godot/data/dex.json', import.meta.url);
writeFileSync(path, JSON.stringify(out, null, 1));
console.log(`wrote ${out.species.length} species, ${skills.length} skills to client-godot/data/dex.json`);
