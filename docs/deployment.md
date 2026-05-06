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
- Go 1.25.9+ installed locally (pinned in `.tool-versions`; bumped from 1.25.0 to clear stdlib CVEs flagged by `govulncheck`)

## Step 1: Build the Binary Locally

From your local repo directory, run the pre-deploy gates and then build for Linux:

```bash
go mod verify                                # confirm go.sum integrity
govulncheck ./...                            # surface known CVEs in code we call
GOOS=linux GOARCH=amd64 go build -o libreshelf .
```

`go mod verify` and `govulncheck` must both exit clean before deploying. If `govulncheck` flags an
issue in the standard library, bump the Go toolchain in `.tool-versions` and `go.mod`. If it flags
a dependency, update that module via `go get -u <module>` and rerun.

The build produces a single self-contained binary. No Go installation is needed on the server.

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
git clone https://github.com/timLP79/LibreShelf.git libreshelf
```

## Step 5: Copy the Binary to EC2

From your local machine:

```bash
scp -i your-key.pem libreshelf ubuntu@<ec2-public-ip>:~/libreshelf/
```

## Step 6: Install and Start the systemd Service

On EC2:

```bash
sudo cp ~/libreshelf/deploy/libreshelf.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable libreshelf
sudo systemctl start libreshelf
sudo systemctl status libreshelf
```

The service runs the app on port 3000 and restarts it automatically on failure or reboot.

## Step 7: Configure nginx

Create the nginx site config:

```bash
sudo nano /etc/nginx/sites-available/libreshelf
```

Paste:

```nginx
server {
    listen 80;
    server_name _;

    # Admin backup import uploads can be tens of MB. nginx's default 1 MB
    # cap returns 413 before the request reaches the Go app. 100 MB is
    # well below the safezip MaxTotalSize cap (500 MB) and well above
    # realistic library backups for a long time.
    client_max_body_size 100M;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

Enable the site and restart nginx:

```bash
sudo ln -s /etc/nginx/sites-available/libreshelf /etc/nginx/sites-enabled/
sudo rm /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl restart nginx
```

## Verification

```bash
# Check Go app is running
sudo systemctl status libreshelf

# Test app directly
curl http://localhost:3000

# Test via nginx
curl http://localhost:80
```

Then visit `http://<ec2-public-ip>` in your browser (no port needed).

## Useful Commands

```bash
# View Go app logs
journalctl -u libreshelf -f

# View nginx logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log

# Restart Go app
sudo systemctl restart libreshelf

# Restart nginx
sudo systemctl restart nginx
```

## Redeploying After Code Changes

```bash
# 1. Pre-deploy gates -- both must exit clean
go mod verify
govulncheck ./...

# 2. Build locally (explicit Linux target for deployment)
GOOS=linux GOARCH=amd64 go build -o libreshelf .

# 3. Stop the service on EC2 first -- scp cannot overwrite a running binary
ssh -i your-key.pem ubuntu@<ec2-public-ip> "sudo systemctl stop libreshelf"

# 4. Pull new templates/static files on EC2
ssh -i your-key.pem ubuntu@<ec2-public-ip> "cd libreshelf && git pull"

# 5. Copy the new binary to EC2
scp -i your-key.pem libreshelf ubuntu@<ec2-public-ip>:~/libreshelf/

# 6. (Optional) Wipe the database to pick up bumped seed passwords --
#    SeedDefaultUsers skips users that already exist, so a password
#    change in source does not propagate to existing rows.
ssh -i your-key.pem ubuntu@<ec2-public-ip> "rm -f ~/libreshelf/data/database.sqlite*"

# 7. Start the service again
ssh -i your-key.pem ubuntu@<ec2-public-ip> "sudo systemctl start libreshelf"
```

> **Note:** The service must be stopped before copying the binary -- Linux will return a
> "dest open: Failure" error if you try to overwrite a running executable. Always stop
> first, copy, then start.
