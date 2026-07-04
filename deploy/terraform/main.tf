# Aurelia: Beastbound — AWS free-tier test environment.
#
# What this provisions (all inside the AWS Free Tier, 12 months):
#   - 1x t3.micro EC2 (750 hrs/month free) running the full stack via Docker
#     Compose: Postgres + Redis + auth + profile + battle + gateway.
#   - A 30 GB gp3 root volume (free tier allows 30 GB).
#   - A security group: SSH from your IP only; gateway (8088) + battle WS (8082)
#     open to the internet so your phone can reach them.
#   - An Elastic IP (free while attached to a running instance) so the address is
#     stable across reboots.
#
# It is deliberately a SINGLE instance with everything in containers — that is the
# cheapest shape that actually runs, not the production design (which is GKE +
# Agones + Cloud SQL; see docs/00-overview.md). This is for testing on a phone.
#
# Cost note: staying on ONE t3.micro 24/7 = 730 hrs/month, under the 750 free
# hours. Stop the instance when unused to be safe. Nothing here uses NAT gateways
# or load balancers (those are NOT free).

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

# --- Networking: reuse the account's default VPC (free; avoids NAT costs) ----
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# --- Latest Amazon Linux 2023 AMI (free-tier eligible, has dnf + docker) ------
data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]
  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-x86_64"]
  }
  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
}

# --- SSH key pair (from your local public key) -------------------------------
resource "aws_key_pair" "deployer" {
  key_name   = "${var.project}-key"
  public_key = file(var.public_key_path)
}

# --- Security group -----------------------------------------------------------
resource "aws_security_group" "app" {
  name        = "${var.project}-sg"
  description = "Beastbound test stack"
  vpc_id      = data.aws_vpc.default.id

  # SSH — locked to your IP only.
  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.my_ip_cidr]
  }

  # Public API gateway (REST) — open so a phone on mobile data can reach it.
  ingress {
    description = "API gateway"
    from_port   = 8088
    to_port     = 8088
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Battle WebSocket server.
  ingress {
    description = "Battle WS"
    from_port   = 8082
    to_port     = 8082
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Project = var.project }
}

# --- EC2 instance -------------------------------------------------------------
resource "aws_instance" "app" {
  ami                         = data.aws_ami.al2023.id
  instance_type               = var.instance_type
  subnet_id                   = data.aws_subnets.default.ids[0]
  key_name                    = aws_key_pair.deployer.key_name
  vpc_security_group_ids      = [aws_security_group.app.id]
  associate_public_ip_address = true

  # Auto-deploy the stack on first boot.
  user_data = templatefile("${path.module}/user_data.sh", {
    repo_url = var.repo_url
  })

  root_block_device {
    volume_size = 30 # GB — free-tier ceiling
    volume_type = "gp3"
  }

  tags = { Project = var.project, Name = "${var.project}-app" }
}

# --- Stable public IP ---------------------------------------------------------
resource "aws_eip" "app" {
  instance = aws_instance.app.id
  domain   = "vpc"
  tags     = { Project = var.project }
}
