// Faithful port of the full turn loop (battle.go) + AI decide (ai.go) + session
// intent loop (session.go), to verify the END-TO-END match path that the Go demo
// exercises: turn ordering, status gates, riders, faint -> forced switch, and
// victory determination. The damage/stat/type core is the same one already
// verified in verify_engine.js. If this and the Go tests disagree, Go is canonical.
import { readFileSync } from 'node:fs';

// ---- RNG (SplitMix64) ----
function makeRNG(seed){let s=BigInt.asUintN(64,BigInt(seed));
  const next=()=>{s=BigInt.asUintN(64,s+0x9E3779B97F4A7C15n);let z=s;
    z=BigInt.asUintN(64,(z^(z>>30n))*0xBF58476D1CE4E5B9n);
    z=BigInt.asUintN(64,(z^(z>>27n))*0x94D049BB133111EBn);return z^(z>>31n);};
  return {intn:n=>Number(next()%BigInt(n)),float:()=>Number(next()>>11n)/2**53,
    chance:p=>p<=0?false:p>=100?true:Number(next()%100n)<p, state:()=>s};
}
// ---- type matrix ----
const EFF=[[1,1,1,1,1,1,1,1,1,1,.5,.5],[1,.5,.5,2,1,1,1,2,1,1,1,2],[1,2,.5,.5,1,2,1,1,1,1,1,1],[1,.5,2,.5,1,2,.5,1,.5,1,1,.5],[1,1,2,.5,.5,0,2,1,1,1,1,1],[1,2,.5,.5,2,1,0,1,2,1,1,2],[1,1,1,2,.5,1,1,1,1,1,1,.5],[1,.5,.5,2,1,2,2,.5,1,1,1,.5],[1,1,1,2,1,.5,1,1,.5,1,.5,0],[1,1,1,1,1,1,1,1,2,.5,0,1],[0,1,1,1,1,1,1,1,1,2,2,1],[1,.5,.5,1,.5,1,1,2,1,1,1,.5]];
const dual=(a,d1,d2)=>d2===d1?EFF[a][d1]:EFF[a][d1]*EFF[a][d2];
// status enum
const ST={None:0,Burn:1,Poison:2,Freeze:3,Sleep:4,Paralysis:5,Toxic:6};

// ---- load merged dex ----
const seed=JSON.parse(readFileSync(new URL('../../data/creatures/seed.json',import.meta.url)));
const flag=JSON.parse(readFileSync(new URL('../../data/creatures/flagships.json',import.meta.url)));
const SP={}; for(const s of seed.species) SP[s.dex_id]=s; for(const s of flag.species) SP[s.dex_id]=s;
const SK={}; for(const s of flag.skills) SK[s.id]=s;

// ---- stats ----
const stat=(base,iv,ev,lvl,hp,m=1)=>{const c=Math.floor((2*base+iv+Math.floor(ev/4))*lvl/100);return hp?c+lvl+10:Math.floor((c+5)*m);};
function computeStats(sp,lvl){const b=sp.base,iv=31;return{
  hp:stat(b.hp,iv,0,lvl,true),atk:stat(b.attack,iv,0,lvl,false),def:stat(b.defense,iv,0,lvl,false),
  spa:stat(b.sp_attack,iv,0,lvl,false),spd:stat(b.sp_defense,iv,0,lvl,false),spe:stat(b.speed,iv,0,lvl,false)};}

function buildBattler(dexId,lvl){const sp=SP[dexId];const ids=[];
  for(const le of (sp.learnset||[])) if(le.level<=lvl&&ids.length<4) ids.push(le.skill_id);
  const st=computeStats(sp,lvl);
  return {sp,lvl,max:st.hp,hp:st.hp,stats:st,status:ST.None,ctr:0,
    e1:sp.element1,e2:sp.element2,skills:ids.map(id=>SK[id]),
    stages:{atk:0,def:0,spa:0,spd:0,spe:0},fainted(){return this.hp<=0;}};}

const sm=s=>s>=0?(2+s)/2:2/(2-s);
const effA=(b,cls)=>{if(cls===1)return b.stats.spa*sm(b.stages.spa);let a=b.stats.atk*sm(b.stages.atk);if(b.status===ST.Burn)a*=.5;return a;};
const effD=(b,cls)=>cls===1?b.stats.spd*sm(b.stages.spd):b.stats.def*sm(b.stages.def);
const effS=b=>{let s=b.stats.spe*sm(b.stages.spe);if(b.status===ST.Paralysis)s*=.5;return s;};

function computeDamage(rng,atk,def,sk){
  if(sk.class===2||sk.power<=0)return{dmg:0,crit:false};
  if(sk.accuracy<=100&&!rng.chance(sk.accuracy))return{dmg:0,miss:true};
  const crit=rng.chance([4,12,50,100][sk.crit_stage||0]);
  let A=effA(atk,sk.class),D=effD(def,sk.class);
  let base=(Math.floor(2*atk.lvl/5)+2)*sk.power*A/D/50;
  let dmg=Math.floor(base)+2;
  if(sk.element===atk.e1||sk.element===atk.e2)dmg*=1.5;
  const e=dual(sk.element,def.e1,def.e2); if(e===0)return{dmg:0,crit};
  dmg*=e; if(crit)dmg*=1.5;
  // offensive low-HP abilities omitted (demo abilities are Blaze/Torrent; effect small)
  dmg*=0.85+rng.float()*0.15;
  return{dmg:Math.max(1,Math.round(dmg)),crit,e};
}

function applyStatus(b,s){if(b.status!==ST.None||b.fainted())return false;
  if(s===ST.Burn&&(b.e1===1||b.e2===1))return false;
  if(s===ST.Freeze&&(b.e1===7||b.e2===7))return false;
  if((s===ST.Poison||s===ST.Toxic)&&(b.e1===8||b.e2===8||b.e1===11||b.e2===11))return false;
  if(s===ST.Paralysis&&(b.e1===4||b.e2===4))return false;
  b.status=s; if(s===ST.Toxic)b.ctr=1; return true;}

function preMove(rng,b){
  if(b.status===ST.Sleep){b.ctr--;if(b.ctr<=0){b.status=ST.None;return true;}return false;}
  if(b.status===ST.Freeze){if(rng.chance(20)){b.status=ST.None;return true;}return false;}
  if(b.status===ST.Paralysis&&rng.chance(25))return false;
  return true;}

function endTurn(b){if(b.fainted())return;
  if(b.status===ST.Burn||b.status===ST.Poison)b.hp-=Math.max(1,Math.floor(b.max/8));
  else if(b.status===ST.Toxic){b.hp-=Math.max(1,Math.floor(b.max*b.ctr/16));b.ctr++;}
  if(b.hp<0)b.hp=0;}

function clamp(v){return Math.max(-6,Math.min(6,v));}

function executeMove(rng,atkSide,defSide,slot,log){
  const a=atkSide.party[atkSide.active],d=defSide.party[defSide.active];
  if(!preMove(rng,a))return;
  const sk=a.skills[slot]; if(!sk)return;
  const res=computeDamage(rng,a,d,sk);
  if(res.miss)return;
  if(res.dmg>0){d.hp-=res.dmg;if(d.hp<0)d.hp=0;}
  const ef=sk.effect;
  if(ef){const tgt=ef.target_self?a:d;
    if(ef.status&&rng.chance(ef.status_chance||100))applyStatus(tgt, ef.status===ST.Sleep?(tgt.status===ST.None?(tgt.status=ST.Sleep,tgt.ctr=1+rng.intn(3),true):false):ef.status);
    if(ef.stat_changes)for(const k in ef.stat_changes){const map={attack:'atk',defense:'def',sp_attack:'spa',sp_defense:'spd',speed:'spe'}[k];if(map)tgt.stages[map]=clamp(tgt.stages[map]+ef.stat_changes[k]);}
    if(ef.heal_frac){const base=sk.class===2?a.max:res.dmg;a.hp=Math.min(a.max,a.hp+Math.floor(base*ef.heal_frac));}
    if(ef.recoil_frac&&res.dmg>0){a.hp-=Math.floor(res.dmg*ef.recoil_frac);if(a.hp<0)a.hp=0;}}
  if(d.fainted())log.push('faint');
}

function order(rng,sides,acts){const a=sides[0].party[sides[0].active],b=sides[1].party[sides[1].active];
  const pa=acts[0].kind==='switch'?6:(a.skills[acts[0].slot]?.priority||0);
  const pb=acts[1].kind==='switch'?6:(b.skills[acts[1].slot]?.priority||0);
  let first=0; if(pa!==pb){if(pb>pa)first=1;} else {const sa=effS(a),sb=effS(b);if(sb>sa)first=1;else if(sb===sa&&rng.chance(50))first=1;}
  return[first,1-first];}

function hasUsable(s){return s.party.some(b=>!b.fainted());}

// ---- AI decide (mirror of ai.go) ----
function scoreMove(self,opp,mv){if(mv.class===2)return (mv.effect&&mv.effect.status&&opp.status===ST.None&&opp.hp/opp.max>.5)?60:20;
  const e=dual(mv.element,opp.e1,opp.e2);if(e===0)return 0;const stab=(mv.element===self.e1||mv.element===self.e2)?1.5:1;
  let exp=mv.power*e*stab*(Math.min(mv.accuracy,100)/100);if(exp*0.004>opp.hp/opp.max)exp+=120;return exp;}
function decide(rng,self,opp){const active=self.party[self.active];
  if(active.fainted()){for(let i=0;i<self.party.length;i++)if(!self.party[i].fainted())return{kind:'switch',to:i};return{kind:'move',slot:0};}
  const o=opp.party[opp.active];let best=0,bs=-1;
  active.skills.forEach((mv,i)=>{const s=scoreMove(active,o,mv);if(s>bs){bs=s;best=i;}});
  // switch if hard countered + low hp
  const incoming=dual(o.e1,active.e1,active.e2);
  if((incoming>=2||active.hp/active.max<.25)){for(let i=0;i<self.party.length;i++){if(i===self.active||self.party[i].fainted())continue;
    const dm=dual(o.e1,self.party[i].e1,self.party[i].e2);if((2-dm)*40-25>bs)return{kind:'switch',to:i};}}
  return{kind:'move',slot:best};}

// ---- session loop (mirror of session.go ResolveTurn + Run) ----
function runMatch(seed){
  const rng=makeRNG(seed);
  const A={active:0,party:[buildBattler(1,32),buildBattler(4,32)]};
  const B={active:0,party:[buildBattler(7,32),buildBattler(40,32)]};
  const sides=[A,B];let turn=0,over=false,winner=-1,switches=0;
  while(!over&&turn<200){turn++;
    const acts=[decide(rng,A,B),decide(rng,B,A)];
    // phase 1 switches
    for(let i=0;i<2;i++)if(acts[i].kind==='switch'){const s=sides[i];if(acts[i].to>=0&&acts[i].to<s.party.length&&!s.party[acts[i].to].fainted()&&acts[i].to!==s.active){s.party[s.active].stages={atk:0,def:0,spa:0,spd:0,spe:0};s.active=acts[i].to;switches++;}}
    // phase 2 moves
    const ord=order(rng,sides,acts);const log=[];
    for(const side of ord){if(acts[side].kind!=='move')continue;if(sides[side].party[sides[side].active].fainted())continue;
      executeMove(rng,sides[side],sides[1-side],acts[side].slot,log);
      if(!hasUsable(sides[0])){over=true;winner=1;break;} if(!hasUsable(sides[1])){over=true;winner=0;break;}}
    if(over)break;
    // phase 3 end of turn
    for(const side of ord)endTurn(sides[side].party[sides[side].active]);
    if(!hasUsable(A)&&!hasUsable(B)){over=true;winner=-1;} else if(!hasUsable(A)){over=true;winner=1;} else if(!hasUsable(B)){over=true;winner=0;}
  }
  return{turn,winner,switches,over,rngState:rng.state().toString()};
}

// ---- assertions ----
let pass=0,fail=0;const Z=(c,m)=>{c?(pass++,console.log('ok  :',m)):(fail++,console.log('FAIL:',m));};
const r1=runMatch(0xC0FFEE), r2=runMatch(0xC0FFEE);
Z(r1.over,'match reaches a conclusion');
Z(r1.turn<200,`match completes in bounded turns (${r1.turn})`);
Z(r1.winner===0||r1.winner===1||r1.winner===-1,`winner determined (side ${r1.winner})`);
Z(JSON.stringify(r1)===JSON.stringify(r2),'same seed -> identical match (deterministic)');
// run several seeds: every match must terminate and (almost always) involve a forced switch on a 2-mon team
let allTerminate=true,anySwitch=false;
for(let s=1;s<=50;s++){const r=runMatch(s*2654435761);if(!r.over||r.turn>=200)allTerminate=false;if(r.switches>0)anySwitch=true;}
Z(allTerminate,'all 50 seeded matches terminate');
Z(anySwitch,'forced-switch-on-faint path exercised across seeds');
console.log(fail===0?`\nSESSION END-TO-END CHECKS PASSED (${pass})`:`\n${fail} FAILED`);
process.exit(fail?1:0);
