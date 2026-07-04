#!/bin/bash
# Cloud-init boot script: installs Docker, adds swap (a t3.micro has only 1 GB
# RAM and building 4 Go services + Postgres would OOM without it), clones the
# repo, and brings up the full stack with Docker Compose. All output is logged to
# /var/log/beastbound-deploy.log so you can watch/debug first boot.
set -euxo pipefail
exec > >(tee -a /var/log/beastbound-deploy.log) 2>&1

echo "=== [1/5] swap (2 GB) so the Go build doesn't OOM on 1 GB RAM ==="
if [ ! -f /swapfile ]; then
  dd if=/dev/zero of=/swapfile bs=1M count=2048
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

echo "=== [2/5] docker + git ==="
dnf update -y
dnf install -y docker git
systemctl enable --now docker
usermod -aG docker ec2-user || true

echo "=== [3/5] docker compose plugin ==="
mkdir -p /usr/local/lib/docker/cli-plugins
curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-x86_64" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

echo "=== [4/5] clone the repo ==="
cd /opt
rm -rf app
git clone "${repo_url}" app
cd app

echo "=== [5/5] build + start the stack ==="
# Build serially to keep peak memory low on the micro instance.
docker compose -f deploy/docker/docker-compose.yml build
docker compose -f deploy/docker/docker-compose.yml up -d

echo "=== DONE. Services should be reachable shortly. ==="
docker compose -f deploy/docker/docker-compose.yml ps
