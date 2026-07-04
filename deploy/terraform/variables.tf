variable "project" {
  description = "Name prefix for all resources"
  type        = string
  default     = "beastbound"
}

variable "region" {
  description = "AWS region. Pick one close to you; all have free tier."
  type        = string
  default     = "ap-south-1" # Mumbai — change to your nearest region
}

variable "instance_type" {
  description = "Free-tier eligible instance. t3.micro (or t2.micro in some regions)."
  type        = string
  default     = "t3.micro"
}

variable "public_key_path" {
  description = "Path to your SSH public key (e.g. ~/.ssh/id_ed25519.pub)"
  type        = string
  default     = "~/.ssh/id_ed25519.pub"
}

variable "my_ip_cidr" {
  description = "Your public IP in CIDR form for SSH access, e.g. 203.0.113.4/32. Get it from https://checkip.amazonaws.com"
  type        = string
}

variable "repo_url" {
  description = "HTTPS git URL of the repo to clone and run on the instance"
  type        = string
  default     = "https://github.com/Rajnikanth31/pokecatch.git"
}
