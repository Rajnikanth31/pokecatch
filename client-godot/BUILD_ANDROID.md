# Build the Android APK (test on your phone)

I can't produce the `.apk` for you from this environment (no Godot/Android SDK
here). But the project is set up so building one is quick. Two paths — pick one.

## Path A — Godot GUI (easiest, ~10 min one-time setup)

**One-time setup**
1. Install **Godot 4.2+** (standard build) — https://godotengine.org/download
2. Install a **JDK 17** (Temurin/Adoptium is fine).
3. Install **Android SDK**. Simplest: install *Android Studio*, open it once so it
   pulls the SDK + build tools. Note the SDK path (e.g. `C:\Users\you\AppData\Local\Android\Sdk`).
4. In Godot: **Editor > Manage Export Templates > Download and Install** (grabs the
   Android export template matching your Godot version).
5. In Godot: **Editor > Editor Settings > Export > Android** — set:
   - *Java SDK Path* → your JDK folder
   - *Android SDK Path* → the SDK folder from step 3
6. Generate a debug keystore (one command, needs the JDK):
   ```powershell
   keytool -keyalg RSA -genkeypair -alias androiddebugkey -keypass android `
     -keystore debug.keystore -storepass android -dname "CN=Android Debug,O=Android,C=US" `
     -validity 9999 -deststoretype pkcs12
   ```
   Point Godot at it in the same Export settings (*Debug Keystore* = this file,
   user `androiddebugkey`, password `android`).

**Build**
1. Open the `client-godot/` folder in Godot (*Import*).
2. **Project > Export…** — the **Android** preset is already defined (see
   `export_presets.cfg`).
3. Click **Export Project**, save as `beastbound.apk`.
4. Copy the APK to your phone (USB, or upload somewhere) and open it. Enable
   *Install unknown apps* for your file manager when prompted.

   …or, with the phone plugged in and USB debugging on, use the **one-click deploy**
   (the little Android/phone icon in Godot's top-right) to build + install + run.

## Path B — Command line / CI (reproducible)

With Godot + templates + Android SDK installed and on PATH:

```powershell
# from client-godot/
godot --headless --export-debug "Android" ../build/beastbound.apk
```

To fully automate in CI, use the community `godot-ci` Docker image (bundles Godot
+ Android SDK + templates); the export command is identical.

## What works on the phone right now

- **Offline:** the title screen, *Play as Guest*, walking the overworld, and the
  UI (collection/team-builder screens render). Placeholder primitive graphics.
- **Online (battles, login, matchmaking):** needs the backend reachable from the
  phone. Run it on your PC (`docker compose -f ../deploy/docker/docker-compose.yml up`)
  and set `ApiClient.BASE_URL` and the battle WS host to your PC's **LAN IP**
  (e.g. `http://192.168.1.50:8088/v1`), with the phone on the same Wi-Fi. Battles
  are server-authoritative, so they don't run without the battle server.

## Reality check

This is an engineering vertical slice, not a finished game — expect coloured
rectangles, not creature art. The APK is for testing that the client runs on a
real device and talks to the backend, not for a polished play session.
