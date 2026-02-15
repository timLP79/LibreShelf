# Deployment Guide: Ubuntu EC2

## Overview

The app is deployed to an Ubuntu EC2 instance as a systemd service. The binary is built
locally and copied to the server via `scp`. nginx sits in front on port 80 and proxies
traffic to the app running on port 3000.

## Architecture

```
Browser → port 80 → nginx → port 3000 → Go app (systemd service)
```

## Prerequisites

- An EC2 instance running Ubuntu
- SSH access via .pem key file
- Port 80 open in the EC2 security group (standard HTTP — open by default in most setups)
- Go 1.24+ installed locally

## Step 1: Build the Binary Locally

From your local repo directory:

```bash
go build -o go-full-stack .
```

This produces a single self-contained binary. No Go installation is needed on the server.

## Step 2: SSH into EC2

```bash
ssh -i your-key.pem ubuntu@<ec2-public-ip>
```

## Step 3: Install Dependencies on EC2

```bash
sudo apt update && sudo apt install -y git nginx
```

## Step 4: Clone the Repo

The repo must be cloned on EC2 for the `templates/` directory — the binary loads templates
from a relative path and must run from the repo directory.

```bash
git clone https://github.com/timLP79/cs408-go-stack.git cs408-go-stack
```

## Step 5: Copy the Binary to EC2

From your local machine:

```bash
scp -i your-key.pem go-full-stack ubuntu@<ec2-public-ip>:~/cs408-go-stack/
```

## Step 6: Install and Start the systemd Service

On EC2:

```bash
sudo cp ~/cs408-go-stack/deploy/go-full-stack.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable go-full-stack
sudo systemctl start go-full-stack
sudo systemctl status go-full-stack
```

The service runs the app on port 3000 and restarts it automatically on failure or reboot.

## Step 7: Configure nginx

Create the nginx site config:

```bash
sudo nano /etc/nginx/sites-available/go-full-stack
```

Paste:

```nginx
server {
    listen 80;
    server_name _;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

Enable the site and restart nginx:

```bash
sudo ln -s /etc/nginx/sites-available/go-full-stack /etc/nginx/sites-enabled/
sudo rm /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl restart nginx
```

## Verification

```bash
# Check Go app is running
sudo systemctl status go-full-stack

# Test app directly
curl http://localhost:3000

# Test via nginx
curl http://localhost:80
```

Then visit `http://<ec2-public-ip>` in your browser (no port needed).

## Useful Commands

```bash
# View Go app logs
journalctl -u go-full-stack -f

# View nginx logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log

# Restart Go app
sudo systemctl restart go-full-stack

# Restart nginx
sudo systemctl restart nginx
```

## Redeploying After Code Changes

```bash
# 1. Build locally
go build -o go-full-stack .

# 2. Copy binary to EC2
scp -i your-key.pem go-full-stack ubuntu@<ec2-public-ip>:~/cs408-go-stack/

# 3. Restart the service on EC2
sudo systemctl restart go-full-stack
```
