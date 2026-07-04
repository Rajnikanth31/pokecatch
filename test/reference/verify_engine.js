/*
 * Reference port of the Go battle engine's pure math (damage, stats, type
 * matrix, RNG). Its job is to independently confirm the formulas the Go unit
 * tests assert on, since the Go toolchain may not be present in every CI image.
 * If this script and the Go tests ever disagree, one of them has a bug.
 *
 *   node verify_engine.js   ->  exits non-zero on any assertion failure.
 */
'use strict';

// --- SplitMix64 RNG (mirror of services/battle/internal/engine/rng.go) -------
function makeRNG(seed) {
  let state = BigInt.asUintN(64, BigInt(seed));
  const M = (1n << 64n) - 1n;
  function next() {
    state = BigInt.asUintN(64, state + 0x9E3779B97F4A7C15n);
    let z = state;
    z = BigInt.asUintN(64, (z ^ (z >> 30n)) * 0xBF58476D1CE4E5B9n);
    z = BigInt.asUintN(64, (z ^ (z >> 27n)) * 0x94D049BB133111EBn);
    return z ^ (z >> 31n);
  }
  return {
    intn: (n) => Number(next() % BigInt(n)),
    float: () => Number(next() >> 11n) / 2 ** 53,
    chance: (pct) => (pct <= 0 ? false : pct >= 100 ? true : Number(next() % 100n) < pct),
  };
}

// --- Element matrix (mirror of pkg/types/types.go) ---------------------------
const E = { Neutral:0,Fire:1,Water:2,Grass:3,Electric:4,Earth:5,Air:6,Ice:7,Toxin:8,Mind:9,Spectre:10,Metal:11 };
const EFF = [
  [1,1,1,1,1,1,1,1,1,1,0.5,0.5],
  [1,0.5,0.5,2,1,1,1,2,1,1,1,2],
  [1,2,0.5,0.5,1,2,1,1,1,1,1,1],
  [1,0.5,2,0.5,1,2,0.5,1,0.5,1,1,0.5],
  [1,1,2,0.5,0.5,0,2,1,1,1,1,1],
  [1,2,0.5,0.5,2,1,0,1,2,1,1,2],
  [1,1,1,2,0.5,1,1,1,1,1,1,0.5],
  [1,0.5,0.5,2,1,2,2,0.5,1,1,1,0.5],
  [1,1,1,2,1,0.5,1,1,0.5,1,0.5,0],
  [1,1,1,1,1,1,1,1,2,0.5,0,1],
  [0,1,1,1,1,1,1,1,1,2,2,1],
  [1,0.5,0.5,1,0.5,1,1,2,1,1,1,0.5],
];
const eff = (a,d)=>EFF[a][d];
const dualEff = (a,d1,d2)=> d2===d1 ? eff(a,d1) : eff(a,d1)*eff(a,d2);

// --- stat formula (mirror of pkg/creatures/stats.go) -------------------------
function computeStat(base, iv, ev, lvl, isHP, mult=1) {
  const core = Math.floor((2*base + iv + Math.floor(ev/4)) * lvl / 100);
  return isHP ? core + lvl + 10 : Math.floor((core + 5) * mult);
}

// --- damage (mirror of damage.go, ability-free path) -------------------------
function computeDamage(rng, atk, def, skill) {
  const res = { damage:0, crit:false, eff:1, stab:1, missed:false };
  if (skill.power <= 0) return res;
  if (skill.accuracy <= 100 && !rng.chance(skill.accuracy)) { res.missed = true; return res; }
  res.crit = rng.chance([4,12,50,100][skill.critStage||0]);
  const A = skill.class==='special' ? atk.spa : atk.atk;
  const D = skill.class==='special' ? def.spd : def.def;
  let base = (Math.floor(2*atk.level/5)+2) * skill.power * A / D / 50;
  let dmg = Math.floor(base) + 2;
  if (atk.types.includes(skill.element)) { res.stab = 1.5; dmg *= 1.5; }
  res.eff = dualEff(skill.element, def.types[0], def.types[1] ?? def.types[0]);
  if (res.eff === 0) { res.damage = 0; return res; }
  dmg *= res.eff;
  if (res.crit) dmg *= 1.5;
  dmg *= 0.85 + rng.float()*0.15;
  res.damage = Math.max(1, Math.round(dmg));
  return res;
}

// --- assertions --------------------------------------------------------------
let failures = 0;
function assert(cond, msg) { if (!cond) { console.error('FAIL:', msg); failures++; } else { console.log('ok  :', msg); } }

// type matrix parity with Go tests
assert(eff(E.Water,E.Fire)===2.0, 'Water>Fire = 2x');
assert(eff(E.Fire,E.Water)===0.5, 'Fire>Water = 0.5x');
assert(eff(E.Electric,E.Earth)===0.0, 'Electric>Earth = immune');
assert(dualEff(E.Grass,E.Water,E.Earth)===4.0, 'Grass>Water/Earth = 4x');

// stat parity
assert(computeStat(100,31,0,100,true)===341, 'HP base100 L100 = 341');
assert(computeStat(100,31,0,100,false)===236, 'Atk base100 L100 = 236');
assert(computeStat(100,31,0,100,false,1.1)===Math.floor(236*1.1), 'Adamant Atk = +10%');

// damage determinism
const fire = { level:50, types:[E.Fire], atk:120, def:80, spa:130, spd:80 };
const grass = { level:50, types:[E.Grass], atk:100, def:90, spa:100, spd:90 };
const ember = { name:'Ember', element:E.Fire, class:'special', power:40, accuracy:100 };
const a = computeDamage(makeRNG(12345), fire, grass, ember);
const b = computeDamage(makeRNG(12345), fire, grass, ember);
assert(a.damage===b.damage && a.crit===b.crit, 'damage deterministic for fixed seed');
assert(a.eff===2.0 && a.stab===1.5, 'STAB Fire>Grass = 1.5x * 2x');

// effectiveness ordering
const water = { level:50, types:[E.Water], atk:100, def:90, spa:120, spd:90 };
const jet = { name:'Jet', element:E.Water, class:'special', power:80, accuracy:100 };
const vsFire  = computeDamage(makeRNG(99), water, {level:50,types:[E.Fire], def:90, spd:90}, jet);
const vsGrass = computeDamage(makeRNG(99), water, {level:50,types:[E.Grass],def:90, spd:90}, jet);
assert(vsFire.damage > vsGrass.damage, 'super-effective > resisted');

console.log(failures === 0 ? '\nALL REFERENCE CHECKS PASSED' : `\n${failures} CHECK(S) FAILED`);
process.exit(failures === 0 ? 0 : 1);
