# Cloudflare Tunnel Setup

Exposes the server via Cloudflare without opening Oracle VCN inbound ports or exposing the origin IP.
No domain required — uses a free `*.trycloudflare.com` URL.

**Note:** URL changes if the VM restarts, but this is a non-issue if the VM stays up long-term.

## Prerequisites

- Oracle instance running
- Cloudflare account (cloudflare.com)

---

## 1. Install cloudflared

```bash
wget https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64
chmod +x cloudflared-linux-amd64
sudo mv cloudflared-linux-amd64 /usr/local/bin/cloudflared
cloudflared --version
```

---

## 2. Create systemd Service

```bash
sudo nano /etc/systemd/system/cloudflared.service
```

Paste:

```ini
[Unit]
Description=Cloudflare Tunnel
After=network.target

[Service]
ExecStart=/usr/local/bin/cloudflared tunnel --url http://localhost:8090
Restart=on-failure
RestartSec=5
User=nobody  # or run `whoami` on the instance to get your username

[Install]
WantedBy=multi-user.target
```

---

## 3. Start and Enable

```bash
sudo systemctl daemon-reload
sudo systemctl enable cloudflared
sudo systemctl start cloudflared
```

---

## 4. Get the URL

```bash
sudo journalctl -u cloudflared | grep "trycloudflare.com"
```

---

## Useful Commands

```bash
sudo journalctl -u cloudflared -f    # live logs
sudo systemctl status cloudflared    # check status
sudo systemctl restart cloudflared   # restart after config change
```

---

## Oracle VCN

No inbound rules needed for HTTP/HTTPS. Only keep:
- SSH (port 22) inbound from your IP

The tunnel is outbound-only from the instance.

---

## Known Warning: ICMP Proxy Disabled

You may see this in logs:

```
WRN The user running cloudflared process has a GID (group ID) that is not within ping_group_range
WRN ICMP proxy feature is disabled
```

This is harmless. ICMP is used for ping/traceroute diagnostics inside the tunnel — not needed for HTTP traffic. The tunnel works fine without it.

