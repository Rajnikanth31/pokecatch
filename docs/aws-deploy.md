# Deploy the backend to AWS Free Tier (step by step)

This stands up the whole backend (Postgres + Redis + auth + profile + battle +
gateway) on **one t3.micro EC2 instance** using Docker Compose, provisioned with
Terraform. It's free for 12 months if you keep to one instance. When it's up,
point the phone app at it.

> This is a **test** environment — a single box with dev secrets and two ports
> open to the internet. Do not put anything real or sensitive on it. The
> production design (GKE + Agones + Cloud SQL) is in `docs/00-overview.md`.

---

## 0. Prerequisites (one-time, ~10 min)

1. **AWS account** with Free Tier (card required, but this stays in free limits).
2. **AWS CLI** installed and configured:
   ```powershell
   aws configure
   # paste an Access Key ID + Secret from AWS Console → IAM → Users → Security credentials
   # default region: your nearest, e.g. ap-south-1
   ```
   Use an IAM user with `AmazonEC2FullAccess` (enough for this), not the root account.
3. **Terraform** installed: https://developer.hashicorp.com/terraform/install
4. **An SSH key**. If you don't have one:
   ```powershell
   ssh-keygen -t ed25519 -C "you@example.com"
   # accept the default path -> creates ~/.ssh/id_ed25519 and id_ed25519.pub
   ```

---

## 1. Configure the deployment

```powershell
cd E:\code\Pokemon\deploy\terraform
copy terraform.tfvars.example terraform.tfvars
notepad terraform.tfvars
```
Fill in three things:
- `region` — your nearest AWS region.
- `public_key_path` — path to your `.pub` key (e.g. `~/.ssh/id_ed25519.pub`).
- `my_ip_cidr` — run `curl https://checkip.amazonaws.com`, add `/32` (e.g. `203.0.113.4/32`).

Make sure your repo is **pushed to GitHub and public** (the instance clones it).
If it's private, you'd add a deploy key — ask me and I'll add that path.

---

## 2. Provision

```powershell
terraform init      # downloads the AWS provider
terraform plan      # review what will be created (1 EC2, 1 SG, 1 EIP, 1 key pair)
terraform apply     # type "yes" to confirm
```

When it finishes it prints outputs:
```
public_ip     = "3.7.x.x"
gateway_url   = "http://3.7.x.x:8088/v1"
battle_ws_url = "ws://3.7.x.x:8082"
ssh_command   = "ssh ec2-user@3.7.x.x"
```

---

## 3. Wait for first boot to build (~10–20 min)

The instance builds the Docker images on boot. Watch it:
```powershell
ssh ec2-user@<public_ip>
sudo tail -f /var/log/beastbound-deploy.log
```
When you see `DONE.` and a table of running services, it's up. Check:
```bash
docker compose -f /opt/app/deploy/docker/docker-compose.yml ps
curl -s localhost:8088/healthz   # from on the box
```

Smoke test from your own machine:
```powershell
curl http://<public_ip>:8088/v1/auth/register `
  -H "content-type: application/json" `
  -d '{\"email\":\"ash@aurelia.gg\",\"password\":\"trainerpass123\",\"display_name\":\"Ash\"}'
```
A JSON token response means the whole stack is live.

---

## 4. Point the phone app at it

In `client-godot/scripts/ApiClient.gd`, set:
```gdscript
const BASE_URL := "http://<public_ip>:8088/v1"
```
and the battle socket to `ws://<public_ip>:8082`. Rebuild the APK (tag a release,
or Actions → android-apk → Run workflow), install on the phone, and it will talk
to your AWS backend over mobile data or Wi-Fi.

---

## 5. Cost control

- **Stop when not testing:** `aws ec2 stop-instances --instance-ids <id>` (or the
  console). A stopped instance costs nothing for compute; the 30 GB EBS is within
  free tier. Start it again when you need it.
- One t3.micro running 24/7 ≈ 730 hrs/month, just under the 750 free hours — but
  only ONE instance, and only for the first 12 months of the account.
- **Tear it all down** when done:
  ```powershell
  terraform destroy
  ```

---

## Troubleshooting

- **Build OOMs / instance unresponsive during first boot:** the 2 GB swap in
  `user_data.sh` normally prevents this; if it still struggles, `terraform apply`
  again with `instance_type = "t3.small"` (small hourly cost outside free tier)
  just for the build, then downsize.
- **Can't SSH:** your public IP changed — update `my_ip_cidr` and re-apply.
- **App not reachable on 8088:** check `docker compose ps` on the box; if a
  service is restarting, `docker compose logs <service>`.
- **Image build fails on go.sum:** already handled — the Dockerfile runs
  `go mod tidy` with `-mod=mod`, so a fresh clone builds without a committed
  `go.sum`.
