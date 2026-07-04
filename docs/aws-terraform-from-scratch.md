# AWS + Terraform, from scratch — complete setup guide

Zero assumptions. By the end you'll have the full Beastbound backend running on a
free AWS instance, reachable from your phone, and you'll know how to stop, redeploy,
and tear it down.

Everything runs in the **AWS Free Tier** (12 months, one small instance). Commands
are for **Windows PowerShell**. Total time: ~30–45 min, most of it waiting.

---

## Contents
0. What you're building
1. Install the tools (AWS CLI, Terraform, Git)
2. Create an AWS account + an IAM user + access keys
3. Connect the AWS CLI to your account
4. Create an SSH key
5. Get the code onto GitHub (the instance clones it)
6. Configure the deployment (`terraform.tfvars`)
7. Run Terraform (`init` → `plan` → `apply`)
8. Verify it's live
9. Point the phone app at it
10. Day-2: stop/start, logs, redeploy after code changes
11. Cost & staying free
12. Tear it all down
13. Troubleshooting
14. How the Terraform files fit together

---

## 0. What you're building

Terraform creates **one `t3.micro` EC2 server** on AWS and, on first boot, it
installs Docker and runs your whole stack with Docker Compose:

```
   phone / PC
       │  http (8088)  ws (8082)
       ▼
 ┌─────────────────────── EC2 t3.micro (Amazon Linux 2023) ───────────────────────┐
 │  gateway :8088   auth :8080   profile :8081   battle :8082                       │
 │  postgres :5432   redis :6379          (all Docker containers)                   │
 └─────────────────────────────────────────────────────────────────────────────────┘
```

Only ports **8088** (API) and **8082** (battle WebSocket) are open to the internet;
SSH (22) is open only to your IP. It's a **test box** — dev secrets, not for real data.

---

## 1. Install the tools

Open **PowerShell as Administrator** and use `winget` (built into Windows 10/11):

```powershell
winget install --id Amazon.AWSCLI -e
winget install --id Hashicorp.Terraform -e
winget install --id Git.Git -e
```

Close and reopen PowerShell (so PATH updates), then confirm all three work:

```powershell
aws --version         # aws-cli/2.x ...
terraform -version    # Terraform v1.x
git --version         # git version 2.x
```

If `terraform` isn't found after reopening, log out/in once, or add
`C:\Program Files\Terraform` to PATH manually.

---

## 2. Create an AWS account + IAM user + access keys

**2a. Account.** If you don't have one: https://aws.amazon.com → *Create an AWS
Account*. You'll enter a card (required), but everything in this guide stays inside
the Free Tier. Pick your nearest region later (this guide defaults to **Mumbai /
ap-south-1**).

**2b. Make an IAM user** (never use root for daily work):
1. AWS Console → search **IAM** → **Users** → **Create user**.
2. Name it `beastbound-admin`. **Don't** enable console access (we only need CLI).
3. Permissions → **Attach policies directly** → tick **AmazonEC2FullAccess**
   (enough for this guide) → Create user.

**2c. Create access keys:**
1. IAM → Users → `beastbound-admin` → **Security credentials** tab.
2. **Access keys** → **Create access key** → use case **Command Line Interface (CLI)**
   → confirm → **Create**.
3. Copy the **Access key ID** and **Secret access key** now (the secret is shown
   once). Keep them private — they're the keys to your account.

---

## 3. Connect the AWS CLI to your account

```powershell
aws configure
```
Paste when prompted:
- **AWS Access Key ID** → from step 2c
- **AWS Secret Access Key** → from step 2c
- **Default region name** → `ap-south-1` (or your nearest)
- **Default output format** → `json`

Verify it's connected:
```powershell
aws sts get-caller-identity
```
You should see your account number and the `beastbound-admin` user ARN. If you get
an auth error, re-run `aws configure` and re-check the keys.

---

## 4. Create an SSH key

This lets you log into the server. Skip if you already have `~/.ssh/id_ed25519.pub`.

```powershell
ssh-keygen -t ed25519 -C "you@example.com"
```
Press Enter to accept the default path and (optionally) set a passphrase. This
creates:
- `C:\Users\<you>\.ssh\id_ed25519`      (private — never share)
- `C:\Users\<you>\.ssh\id_ed25519.pub`  (public — Terraform uploads this)

---

## 5. Get the code onto GitHub

The instance **clones your repo** on boot, so it must be pushed and reachable.

```powershell
cd E:\code\Pokemon
git add .
git commit -m "Deploy config"
git push
```

The repo must be **public** for the simple clone to work (default). If it's private,
tell me and I'll add the deploy-key path to the Terraform.

> The `repo_url` defaults to `https://github.com/Rajnikanth31/pokecatch.git` — change
> it in the next step if yours differs.

---

## 6. Configure the deployment

```powershell
cd E:\code\Pokemon\deploy\terraform
Copy-Item terraform.tfvars.pokemon terraform.tfvars
notepad terraform.tfvars
```

Set these three values:

| Variable | What to put | How to get it |
|---|---|---|
| `region` | `"ap-south-1"` (or nearest) | pick from AWS regions |
| `public_key_path` | `"~/.ssh/id_ed25519.pub"` | the `.pub` from step 4 |
| `my_ip_cidr` | `"<your-ip>/32"` | run `curl https://checkip.amazonaws.com`, append `/32` |

Example finished `terraform.tfvars`:
```hcl
region          = "ap-south-1"
public_key_path = "~/.ssh/id_ed25519.pub"
my_ip_cidr      = "203.0.113.4/32"
repo_url        = "https://github.com/Rajnikanth31/pokecatch.git"
```

Save and close.

---

## 7. Run Terraform

Still in `deploy/terraform`:

**7a. Initialize** (downloads the AWS provider — once per machine):
```powershell
terraform init
```
Expect: `Terraform has been successfully initialized!`

**7b. Preview** what will be created:
```powershell
terraform plan
```
Expect: `Plan: 4 to add, 0 to change, 0 to destroy.` (key pair, security group,
EC2 instance, Elastic IP). If it errors here, it's usually credentials (step 3) or a
bad `my_ip_cidr`/key path (step 6).

**7c. Create it:**
```powershell
terraform apply
```
Type `yes` when prompted. After ~1–2 min it prints outputs:
```
battle_ws_url = "ws://3.7.x.x:8082"
gateway_url   = "http://3.7.x.x:8088/v1"
public_ip     = "3.7.x.x"
ssh_command   = "ssh ec2-user@3.7.x.x"
```
**Save that `public_ip`.** The server now exists, but it's still building the app
(next step).

---

## 8. Verify it's live

First boot builds the Docker images on the box (~10–20 min on a t3.micro). Watch it:

```powershell
ssh ec2-user@<public_ip>
# on the server:
sudo tail -f /var/log/beastbound-deploy.log
```
Wait for `=== DONE.` and a table of running containers. `Ctrl+C` to stop tailing,
then check health on the box:
```bash
docker compose -f /opt/app/deploy/docker/docker-compose.yml ps
curl -s localhost:8088/healthz && echo OK
exit
```

Now test from **your** machine (proves the internet-facing path works):
```powershell
curl http://<public_ip>:8088/v1/auth/register `
  -H "content-type: application/json" `
  -d '{\"email\":\"ash@aurelia.gg\",\"password\":\"trainerpass123\",\"display_name\":\"Ash\"}'
```
A JSON response with `access_token` = the entire backend is live. 🎉

---

## 9. Point the phone app at it

Edit `client-godot/scripts/ApiClient.gd`:
```gdscript
const BASE_URL := "http://<public_ip>:8088/v1"
```
For battles, use `ws://<public_ip>:8082`. Rebuild the APK (push a tag, or Actions →
android-apk → Run workflow), install on the phone, and it talks to AWS over Wi-Fi or
mobile data.

---

## 10. Day-2 operations

**Stop the server when not testing (saves free-tier hours):**
```powershell
aws ec2 stop-instances --instance-ids <instance-id>
aws ec2 start-instances --instance-ids <instance-id>   # start again later
```
Get the instance id from `terraform output` or the EC2 console. (The Elastic IP
stays the same across stop/start.)

**SSH in:**
```powershell
ssh ec2-user@<public_ip>
```

**See logs / restart a service:**
```bash
cd /opt/app
docker compose -f deploy/docker/docker-compose.yml logs -f gateway
docker compose -f deploy/docker/docker-compose.yml restart profile
```

**Redeploy after you change code** (push to GitHub first, then on the box):
```bash
cd /opt/app
git pull
docker compose -f deploy/docker/docker-compose.yml up -d --build
```

---

## 11. Cost & staying free

- **One** t3.micro running 24/7 ≈ 730 hrs/month, just under the **750 free hours**
  — but only ONE instance and only for the account's first 12 months.
- 30 GB EBS and the Elastic IP (while attached to a running instance) are free.
- **Stop the instance when idle** to be safe. A stopped instance costs $0 for compute.
- Nothing here uses load balancers or NAT gateways (those are **not** free).
- Set a billing alarm: AWS Console → **Billing** → **Budgets** → create a $1 alert.

---

## 12. Tear it all down

When you're done, remove everything so nothing can ever bill you:
```powershell
cd E:\code\Pokemon\deploy\terraform
terraform destroy      # type "yes"
```
Expect `Destroy complete! Resources: 4 destroyed.` That's it — the instance, IP,
security group, and key pair are gone.

---

## 13. Troubleshooting

| Symptom | Fix |
|---|---|
| `terraform plan` → credentials error | Re-run `aws configure`; check keys; `aws sts get-caller-identity`. |
| `plan` → "InvalidKeyPair" / key path error | `public_key_path` must point to your `.pub` file; the file must exist. |
| Can't SSH (`Connection timed out`) | Your public IP changed. Update `my_ip_cidr` in tfvars, `terraform apply` again. |
| App not answering on 8088 | Build may still be running — tail `/var/log/beastbound-deploy.log`. Then `docker compose ps`. |
| A container keeps restarting | `docker compose logs <service>` on the box; usually Postgres still starting — wait, it retries. |
| Build OOMs on first boot | The 2 GB swap normally prevents it; if not, temporarily set `instance_type="t3.small"` and re-apply. |
| `go.sum` build error | Already handled — the Dockerfile runs `go mod tidy -mod=mod`, so a fresh clone builds. |
| Region has no t3.micro free tier | Set `instance_type="t2.micro"` in tfvars. |

---

## 14. How the Terraform files fit together

Everything lives in `deploy/terraform/`:

| File | Role |
|---|---|
| `main.tf` | The infrastructure: finds the default VPC + latest Amazon Linux AMI, creates the SSH key pair, security group, EC2 instance, and Elastic IP. |
| `variables.tf` | The knobs (region, instance type, key path, your IP, repo URL) with defaults. |
| `terraform.tfvars` | **Your** values (gitignored — never committed). Created from `terraform.tfvars.pokemon`. |
| `outputs.tf` | What gets printed after apply (IP, URLs, ssh command). |
| `user_data.sh` | The boot script the instance runs once: swap → Docker → clone repo → `docker compose up`. |

Mental model: **`variables.tf` = the form**, **`terraform.tfvars` = your answers**,
**`main.tf` = what gets built**, **`user_data.sh` = what runs inside the box**,
**`outputs.tf` = the receipt**.

---

### Quick reference (the whole thing, once set up)

```powershell
# one-time
aws configure
ssh-keygen -t ed25519

# each deploy
cd E:\code\Pokemon\deploy\terraform
Copy-Item terraform.tfvars.pokemon terraform.tfvars   # edit it
terraform init
terraform apply            # -> note the public_ip
ssh ec2-user@<ip> "sudo tail -f /var/log/beastbound-deploy.log"   # wait for DONE

# when finished
terraform destroy
```
