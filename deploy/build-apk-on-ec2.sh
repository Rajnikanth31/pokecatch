#!/bin/bash
# Build the Android APK ON the EC2 instance (no GitHub Actions needed).
# Installs Godot 4.2.2 + export templates + JDK 17 + Android SDK, points the
# client at this server, and exports a debug APK to ~/beastbound.apk.
#
# Run on the EC2 box:   bash build-apk-on-ec2.sh
# Downloads ~2-3 GB and takes ~15-20 min the first time. The 30 GB disk has room.
set -euxo pipefail

SERVER_IP="15.207.157.71"                 # this instance's public IP
GODOT=4.2.2-stable
GVER=4.2.2.stable

echo "=== [1/7] JDK 17 + tools ==="
sudo dnf install -y java-17-amazon-corretto unzip wget which
JAVA_HOME="$(dirname "$(dirname "$(readlink -f "$(which java)")")")"
echo "JAVA_HOME=$JAVA_HOME"

echo "=== [2/7] Godot editor ==="
cd ~
wget -q "https://github.com/godotengine/godot/releases/download/${GODOT}/Godot_v${GODOT}_linux.x86_64.zip"
unzip -o "Godot_v${GODOT}_linux.x86_64.zip"
chmod +x "Godot_v${GODOT}_linux.x86_64"
sudo mv "Godot_v${GODOT}_linux.x86_64" /usr/local/bin/godot

echo "=== [3/7] Godot export templates ==="
wget -q "https://github.com/godotengine/godot/releases/download/${GODOT}/Godot_v${GODOT}_export_templates.tpz"
rm -rf /tmp/tpl && mkdir -p /tmp/tpl
unzip -o "Godot_v${GODOT}_export_templates.tpz" -d /tmp/tpl
mkdir -p "$HOME/.local/share/godot/export_templates/${GVER}"
mv /tmp/tpl/templates/* "$HOME/.local/share/godot/export_templates/${GVER}/"

echo "=== [4/7] Android SDK ==="
cd ~
wget -q "https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip" -O cmdline.zip
mkdir -p "$HOME/android-sdk/cmdline-tools"
unzip -o cmdline.zip -d "$HOME/android-sdk/cmdline-tools"
rm -rf "$HOME/android-sdk/cmdline-tools/latest"
mv "$HOME/android-sdk/cmdline-tools/cmdline-tools" "$HOME/android-sdk/cmdline-tools/latest"
export ANDROID_HOME="$HOME/android-sdk"
export PATH="$PATH:$ANDROID_HOME/cmdline-tools/latest/bin"
yes | sdkmanager --licenses >/dev/null 2>&1 || true
sdkmanager "platform-tools" "build-tools;34.0.0" "platforms;android-34"

echo "=== [5/7] debug keystore ==="
keytool -keyalg RSA -genkeypair -alias androiddebugkey -keypass android \
  -keystore "$HOME/debug.keystore" -storepass android \
  -dname "CN=Android Debug,O=Android,C=US" -validity 9999 -deststoretype pkcs12 || true

echo "=== [6/7] Godot editor settings (SDK + Java + keystore) ==="
mkdir -p "$HOME/.config/godot"
cat > "$HOME/.config/godot/editor_settings-4.tres" <<EOF
[gd_resource type="EditorSettings" format=3]
[resource]
export/android/android_sdk_path = "$HOME/android-sdk"
export/android/java_sdk_path = "$JAVA_HOME"
export/android/debug_keystore = "$HOME/debug.keystore"
export/android/debug_keystore_user = "androiddebugkey"
export/android/debug_keystore_pass = "android"
EOF

echo "=== [7/7] point client at this server + export APK ==="
# Work in a home copy so we don't need root on /opt/app.
rm -rf "$HOME/client-godot"
cp -r /opt/app/client-godot "$HOME/client-godot"
# Force the API URL to this server (idempotent).
sed -i "s#const BASE_URL := \".*\"#const BASE_URL := \"http://${SERVER_IP}:8088/v1\"#" \
  "$HOME/client-godot/scripts/ApiClient.gd"
grep BASE_URL "$HOME/client-godot/scripts/ApiClient.gd"

export ANDROID_HOME="$HOME/android-sdk"
# Import pass warms the resource cache, then export the debug APK.
godot --headless --editor --path "$HOME/client-godot" --quit || true
godot --headless --path "$HOME/client-godot" --export-debug "Android" "$HOME/beastbound.apk"

ls -lh "$HOME/beastbound.apk"
echo ""
echo "APK ready at ~/beastbound.apk"
echo "Download it to your PC (run this ON YOUR PC):"
echo "   scp ec2-user@${SERVER_IP}:~/beastbound.apk ."
