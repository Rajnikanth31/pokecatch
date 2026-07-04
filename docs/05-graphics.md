# 05 — Graphics & Art Direction

## Style decision: stylized 2.5D

**Chosen: 2.5D** (3D environments and creatures with an orthographic-ish camera and
hand-stylized, painterly textures). Rationale and tradeoff:

- vs **pixel art:** richer creature expression and battle VFX, scales better to large
  modern displays, and easier to animate 300+ creatures via skeletal rigs than via
  hand-drawn frames.
- vs **full 3D stylized:** far cheaper to hit 60 FPS on mid-tier mobile and to export to
  **WebGL**, and a fixed-ish camera lets us fake lighting/detail with baked work instead
  of expensive realtime. We accept reduced camera freedom.

The look: clean silhouettes, type-coded color language (also encoded as shape for
colorblind accessibility), soft rim light, and a "restoration" palette that desaturates
in Sundered areas and saturates as you heal them — art directly serving the theme.

## Asset pipeline

- **Character models:** modular Warden rig (swappable cosmetic meshes for monetization)
  on a shared humanoid skeleton; one animation set retargets to all.
- **Creature models:** authored against a small set of body archetypes (biped, quadruped,
  serpentine, floater, swarm) so 300 creatures share rigs/animation sets — essential to
  produce that volume affordably. Flagships get bespoke rigs.
- **UI assets:** 9-slice scalable panels + SDF icons (resolution-independent, one set for
  all platforms).
- **VFX:** GPU-particle templates keyed by element + damage class; new skills reuse
  templates (data-driven), so content scales without new VFX per move.
- **Battle animations:** skeletal state machines (idle/attack/hit/faint/special) with
  additive hit-reactions; signature flagship moves get unique sequences.
- **Environment assets:** modular biome kits (tilesets of 3D props) for fast level
  building; instanced rendering for vegetation/rocks.
- **Lighting pipeline:** baked lightmaps for static geometry + a few realtime lights for
  hero moments; time-of-day handled by a gradient/skybox system rather than fully
  dynamic GI (mobile/web budget).
- **Shader design:** a shared stylized PBR-lite shader (ramp lighting + rim), a
  dissolve/restore shader for the heal-the-world effect, type-aura shaders, and a
  cheap water shader with depth fade for Tidewreck.

## Optimization targets

- **Mobile: 60 FPS** — aggressive LODs, texture atlasing, draw-call batching (instancing),
  virtualized UI, half-resolution particles, and a quality-tier auto-detect.
- **PC: 120 FPS** — higher LODs, MSAA, optional dynamic shadows; uncapped with vsync
  option.
- **Web (WebGL):** a reduced tier (compressed textures, fewer realtime lights) targeting
  60 FPS on desktop browsers.

Budgets: ~150k tris on screen (mobile), <120 draw calls/frame (mobile) via batching,
≤256 MB texture memory (mobile tier). Enforced in CI via a scene-stats lint on the Godot
project.
