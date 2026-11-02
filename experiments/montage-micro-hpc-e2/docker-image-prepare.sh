#!/bin/sh
set -eu
# Execute only through console API on the disposable AKO-136 builder.
test "$(id -u)" = 0
test -f /etc/akoflow-disposable-image-builder
test "$(cat /etc/akoflow-disposable-image-builder)" = AKO-136
test -d /akoflow/workspace
test -z "$(find /akoflow/workspace -mindepth 1 -maxdepth 1 -print -quit)"
test -z "$(docker ps -aq)"
docker version >/dev/null
for tool in rsync tar python3 sha256sum; do command -v "$tool" >/dev/null; done
systemctl is-active --quiet docker
systemctl is-active --quiet google-guest-agent

python3 - <<'PY'
import json, subprocess
from pathlib import Path
packages = subprocess.check_output([
    'dpkg-query', '-W', '-f=${Package}=${Version}\n',
    'docker.io', 'rsync', 'tar', 'python3', 'google-guest-agent'
], text=True).splitlines()
Path('/etc/akoflow-worker-image.json').write_text(json.dumps({
    'schemaVersion': 1, 'issue': 'AKO-136', 'os': 'ubuntu-24.04',
    'packages': sorted(packages), 'workspaceEmpty': True,
    'dockerEnabled': True, 'uniqueHostKeysOnBoot': True
}, sort_keys=True) + '\n')
Path('/etc/systemd/system/akoflow-image-hostkeys.service').write_text('''[Unit]
Description=Generate unique SSH host keys for a cloned Akoflow worker
After=local-fs.target
Before=ssh.service google-guest-agent.service google-startup-scripts.service

[Service]
Type=oneshot
ExecStart=/usr/bin/ssh-keygen -A
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
''')
PY
systemctl daemon-reload
systemctl enable akoflow-image-hostkeys.service
cloud-init clean --logs --machine-id --seed
rm -f -- /etc/akoflow-disposable-image-builder
# No remote activity data exists: explicit host identity files of this builder only.
for key in /etc/ssh/ssh_host_rsa_key /etc/ssh/ssh_host_rsa_key.pub \
           /etc/ssh/ssh_host_ecdsa_key /etc/ssh/ssh_host_ecdsa_key.pub \
           /etc/ssh/ssh_host_ed25519_key /etc/ssh/ssh_host_ed25519_key.pub; do
    rm -f -- "$key"
done
sync
sha256sum /etc/akoflow-worker-image.json
printf 'IMAGE_PREPARED\n'
