// Runnable mirror of tools/creaturegen/main.go used to materialize the committed
// seed in environments without a Go toolchain. Algorithm is identical; if the Go
// and JS outputs ever diverge, the Go version is canonical.
//   node generate.mjs --seed 42 --count 300 --out ../../data/creatures/seed.json
import { writeFileSync } from 'node:fs';

const args = Object.fromEntries(process.argv.slice(2).reduce((a, v, i, arr) =>
  v.startsWith('--') ? [...a, [v.slice(2), arr[i + 1]]] : a, []));
const SEED = BigInt(args.seed ?? 42);
const COUNT = Number(args.count ?? 300);
const OUT = args.out ?? new URL('../../data/creatures/seed.json', import.meta.url).pathname;

const EL = ['Neutral','Fire','Water','Grass','Electric','Earth','Air','Ice','Toxin','Mind','Spectre','Metal'];
const RAR = ['Common','Uncommon','Rare','Epic','Legendary','Mythic'];
const bst = { Common:[290,360], Uncommon:[340,430], Rare:[410,500], Epic:[480,540], Legendary:[560,600], Mythic:[580,640] };
const archetypes = [
  ['sweeper',   {hp:8, attack:16,defense:7, sp_attack:10,sp_defense:7, speed:16}],
  ['specialist',{hp:9, attack:6, defense:8, sp_attack:18,sp_defense:10,speed:13}],
  ['tank',      {hp:18,attack:12,defense:16,sp_attack:7, sp_defense:13,speed:6}],
  ['wall',      {hp:16,attack:6, defense:18,sp_attack:8, sp_defense:18,speed:6}],
  ['bruiser',   {hp:13,attack:17,defense:13,sp_attack:8, sp_defense:11,speed:10}],
  ['balanced',  {hp:12,attack:12,defense:12,sp_attack:12,sp_defense:12,speed:12}],
];
const biomes = ['verdant_meadow','emberfall_volcano','tidewreck_coast','stormspire_peaks',
  'hollow_mire','frostbarrow_tundra','sunken_archive','whispering_canopy','obsidian_caldera','aurora_expanse'];

let s = SEED | 1n;
const next = () => { s ^= BigInt.asUintN(64, s << 13n); s ^= s >> 7n; s ^= BigInt.asUintN(64, s << 17n); s = BigInt.asUintN(64, s); return s; };
const intn = (n) => Number(next() % BigInt(n));
const span = (lo, hi) => hi <= lo ? lo : lo + intn(hi - lo + 1);

const rarityFor = (i, total) =>
  i >= total-6  ? 5 : i >= total-18 ? 4 : i >= total-60 ? 3 : i >= total-130 ? 2 : i >= total-220 ? 1 : 0;
const catchRate = (r) => [200,140,90,45,12,3][r];

function distribute(budget, w) {
  const total = Object.values(w).reduce((a,b)=>a+b,0);
  const sc = (x) => 10 + Math.floor(x*(budget-60)/total);
  return { hp:sc(w.hp), attack:sc(w.attack), defense:sc(w.defense), sp_attack:sc(w.sp_attack), sp_defense:sc(w.sp_defense), speed:sc(w.speed) };
}

const dex = [];
for (let i = 0; i < COUNT; i++) {
  const rarity = rarityFor(i, COUNT);
  const [lo, hi] = bst[RAR[rarity]];
  const budget = span(lo, hi);
  const [name, w] = archetypes[intn(archetypes.length)];
  const e1 = 1 + intn(EL.length - 1);
  const e2 = intn(100) < 45 ? intn(EL.length) : e1;
  dex.push({
    dex_id: i+1, name: `${name}-${String(i+1).padStart(3,'0')}`,
    element1: e1, element2: e2, base: distribute(budget, w),
    rarity, abilities: [], learnset: [], evolves_to_id: 0, evolve_level: 0,
    catch_rate: catchRate(rarity), xp_yield: 60 + Math.floor(budget/6),
    spawns: [biomes[intn(biomes.length)]],
  });
}
for (let i = 0; i+2 < dex.length && i < 180; i += 3) {
  if (dex[i].rarity <= 2) {
    dex[i].evolves_to_id = dex[i+1].dex_id; dex[i].evolve_level = 16;
    dex[i+1].evolves_to_id = dex[i+2].dex_id; dex[i+1].evolve_level = 34;
    dex[i+1].element1 = dex[i].element1; dex[i+2].element1 = dex[i].element1;
  }
}

const out = { version: 1, seed: Number(SEED), generated: COUNT, species: dex };
writeFileSync(OUT, JSON.stringify(out, null, 2));
// Quick balance audit: confirm no creature breaks its rarity BST ceiling.
let violations = 0;
for (const c of dex) {
  const total = Object.values(c.base).reduce((a,b)=>a+b,0);
  if (total > bst[RAR[c.rarity]][1] + 6) violations++; // +6 rounding slack
}
console.log(`wrote ${dex.length} species to ${OUT}; BST ceiling violations: ${violations}`);
