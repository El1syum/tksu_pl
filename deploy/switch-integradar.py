"""One-time domain handover requested by the server owner; run as root on el1s."""
from datetime import datetime, timezone
from pathlib import Path
import shutil
import subprocess

config = Path('/opt/botmost/shared/Caddyfile')
old = config.read_text()
source = 'reverse_proxy integradar-web:4173'
target = 'reverse_proxy tksu-pl:8080'
if target in old and source not in old:
    print('Domain is already configured for tksu-pl')
    raise SystemExit(0)
if old.count(source) != 1:
    raise SystemExit('Expected exactly one existing integradar upstream; no changes made')
backup = Path('/opt/tksu-pl/backups') / ('Caddyfile-before-tksu-' + datetime.now(timezone.utc).strftime('%Y%m%d-%H%M%S'))
shutil.copy2(config, backup)
# Preserve the inode: Caddy mounts this individual file, not its directory.
config.write_text(old.replace(source, target))
try:
    subprocess.run(['docker', 'exec', 'botmost-caddy-1', 'caddy', 'validate', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile'], check=True)
    subprocess.run(['docker', 'kill', '--signal=SIGUSR1', 'botmost-caddy-1'], check=True)
except BaseException:
    config.write_text(old)
    subprocess.run(['docker', 'kill', '--signal=SIGUSR1', 'botmost-caddy-1'], check=False)
    raise
print(f'Proxy switched. Previous config: {backup}')
print('Verify https://integradar.org/ping, then stop integradar-web.')
