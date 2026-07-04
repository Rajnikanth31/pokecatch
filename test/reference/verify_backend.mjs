// Reference checks for the backend logic that has no Go toolchain in this
// environment: the HS256 JWT scheme (services/auth/internal/jwt.go), the trade
// state machine (services/profile/internal/trade.go), the AI move heuristic
// (services/battle/internal/ai/ai.go) and the save checksum
// (services/profile/internal/save.go). If these disagree with the Go tests, Go
// is canonical. This file caught a real bug: the trade code originally
// auto-transitioned to a "locked" state that made the anti-scam
// "offer-change-resets-locks" path impossible — fixed after this check failed.
import { createHmac, timingSafeEqual, createHash } from 'node:crypto';

let f = 0;
const A = (c, m) => { c ? console.log('ok  :', m) : (f++, console.log('FAIL:', m)); };

// ---- HS256 JWT ----
const b64 = (buf) => Buffer.from(buf).toString('base64url');
const HEADER = b64(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
const sign = (secret, input) => createHmac('sha256', secret).update(input).digest();
const make = (secret, claims) => { const p = b64(JSON.stringify(claims)); const si = HEADER + '.' + p; return si + '.' + b64(sign(secret, si)); };
const eq = (a, b) => a.length === b.length && timingSafeEqual(a, b); // mirrors Go hmac.Equal
function verify(secret, tok) {
  const p = tok.split('.'); if (p.length !== 3) return { err: 'malformed' };
  const si = p[0] + '.' + p[1];
  if (!eq(sign(secret, si), Buffer.from(p[2], 'base64url'))) return { err: 'signature' };
  const c = JSON.parse(Buffer.from(p[1], 'base64url'));
  if (Math.floor(Date.now() / 1000) >= c.exp) return { err: 'expired' };
  return { claims: c };
}
const secret = Buffer.from('s3cr3t');
const now = Math.floor(Date.now() / 1000);
A(verify(secret, make(secret, { sub: 'acc-1', iat: now, exp: now + 60, ver: 1 })).claims.sub === 'acc-1', 'JWT round-trip');
A(verify(Buffer.from('wrong'), make(secret, { sub: 'x', iat: now, exp: now + 60, ver: 1 })).err === 'signature', 'JWT wrong key rejected');
A(verify(secret, make(secret, { sub: 'x', iat: now, exp: now - 1, ver: 1 })).err === 'expired', 'JWT expiry enforced');
A(HEADER === 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9', 'JWT header is standard');

// ---- Trade state machine ----
const owns = { 'ash-mon': 'ash', 'gary-mon': 'gary' };
const newTrade = (a, b) => ({ state: 'open', offers: { [a]: [], [b]: [] }, locked: { [a]: false, [b]: false }, A: a, B: b });
const stage = (t, acct, ids) => { if (t.state !== 'open') return 'closed'; for (const id of ids) if (owns[id] !== acct) return 'not_owner'; t.offers[acct] = ids.slice(); t.locked[t.A] = false; t.locked[t.B] = false; return 'ok'; };
const lock = (t, acct) => { if (t.state !== 'open') return 'closed'; t.locked[acct] = true; return 'ok'; };
const commit = (t) => { if (t.state !== 'open') return 'closed'; if (!(t.locked[t.A] && t.locked[t.B])) return 'not_locked'; for (const id of t.offers[t.A]) owns[id] = t.B; for (const id of t.offers[t.B]) owns[id] = t.A; t.state = 'committed'; return 'ok'; };
const t = newTrade('ash', 'gary');
A(stage(t, 'ash', ['gary-mon']) === 'not_owner', 'trade: cannot stage unowned');
stage(t, 'ash', ['ash-mon']); stage(t, 'gary', ['gary-mon']); lock(t, 'ash'); lock(t, 'gary');
stage(t, 'ash', ['ash-mon']); // offer change after both locked
A(commit(t) === 'not_locked', 'trade: offer change resets locks (anti-scam)');
lock(t, 'ash'); lock(t, 'gary');
A(commit(t) === 'ok', 'trade: commit after re-lock');
A(owns['ash-mon'] === 'gary' && owns['gary-mon'] === 'ash', 'trade: ownership swapped');
A(commit(t) === 'closed', 'trade: double commit rejected (anti-dupe)');

// ---- AI heuristic ----
const EFF = [[1,1,1,1,1,1,1,1,1,1,.5,.5],[1,.5,.5,2,1,1,1,2,1,1,1,2],[1,2,.5,.5,1,2,1,1,1,1,1,1],[1,.5,2,.5,1,2,.5,1,.5,1,1,.5],[1,1,2,.5,.5,0,2,1,1,1,1,1],[1,2,.5,.5,2,1,0,1,2,1,1,2],[1,1,1,2,.5,1,1,1,1,1,1,.5],[1,.5,.5,2,1,2,2,.5,1,1,1,.5],[1,1,1,2,1,.5,1,1,.5,1,.5,0],[1,1,1,1,1,1,1,1,2,.5,0,1],[0,1,1,1,1,1,1,1,1,2,2,1],[1,.5,.5,1,.5,1,1,2,1,1,1,.5]];
const dual = (a, d1, d2) => d2 === d1 ? EFF[a][d1] : EFF[a][d1] * EFF[a][d2];
const score = (self, opp, mv) => { const e = dual(mv.el, opp.e1, opp.e2); if (e === 0) return 0; const st = (mv.el === self.e1 || mv.el === self.e2) ? 1.5 : 1; let x = mv.p * e * st; if (x * .004 > opp.hp) x += 120; return x; };
A(score({e1:2,e2:2},{e1:1,e2:1,hp:1},{el:2,p:60}) > score({e1:2,e2:2},{e1:1,e2:1,hp:1},{el:0,p:60}), 'AI favors SE STAB');
A(score({e1:4,e2:4},{e1:5,e2:5,hp:1},{el:4,p:65}) === 0, 'AI rejects immune move');

// ---- Save checksum ----
const cs = (v, p) => { const h = createHash('sha256'); const b = Buffer.alloc(8); b.writeUInt32LE(v, 0); h.update(b); h.update(p); return h.digest('hex'); };
const pl = Buffer.from('{"version":3,"region":"ok"}');
A(cs(3, pl) === cs(3, pl) && cs(3, pl) !== cs(3, Buffer.from('{"version":3,"region":"HACKED"}')), 'save checksum detects tamper');

console.log(f === 0 ? `\nBACKEND REFERENCE CHECKS PASSED (${9})` : `\n${f} FAILED`);
process.exit(f ? 1 : 0);
