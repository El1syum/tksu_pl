# Эксплуатация на el1s

## Текущая схема

`https://integradar.org` → `botmost-caddy-1` → Docker DNS `tksu-pl:8080` → `/data/expenses.db`.

TLS-сертификат и редирект `www.integradar.org` обслуживает существующий Caddy. В его конфигурации изменена только строка upstream для integradar: `reverse_proxy integradar-web:4173` → `reverse_proxy tksu-pl:8080`. Контейнер `integradar-web` остановлен по указанию владельца. Его каталог `/opt/integradar/current` и сам контейнер сохранены.

Первоначальная конфигурация Caddy сохранена в `/opt/tksu-pl/backups/Caddyfile-before-tksu-20260929-220749`.

Административный API Caddy отключён. Конфигурация проверяется через `caddy validate`, затем перечитывается сигналом `SIGUSR1`, который [поддерживает установленная версия Caddy 2.11.4](https://github.com/caddyserver/caddy/blob/v2.11.4/sigtrap_posix.go). Общий прокси не перезапускается.

## Обновление

1. Убедиться, что изменения сохранены в Git.
2. Запустить `./deploy/deploy.ps1 -Server el1s` с локальной машины.
3. Проверить `https://integradar.org/ping`, страницу входа и статус Docker healthcheck.

Каждый выпуск хранится отдельно. `/opt/tksu-pl/current` указывает на активный выпуск, `/opt/tksu-pl/shared/previous-release` — на предшествующий. Старые образы не удаляются автоматически. Программа, шаблоны и frontend встроены в единый бинарник.

## Копия базы вручную

Не копируйте только `.db` работающего приложения обычным `cp`: в WAL могут находиться ещё не перенесённые транзакции. Используйте SQLite Backup API; Python 3 доступен на сервере.

В SSH-сеансе:

```sh
sudo python3 - <<'PY'
import sqlite3
from datetime import datetime, timezone
from pathlib import Path
target = Path('/opt/tksu-pl/backups') / ('manual-' + datetime.now(timezone.utc).strftime('%Y%m%d-%H%M%S') + '.db')
with sqlite3.connect('/opt/tksu-pl/shared/data/expenses.db') as source, sqlite3.connect(target) as backup:
    source.backup(backup)
target.chmod(0o600)
print(target)
PY
```

Секретный `.env` нужно хранить отдельно от репозитория и вместе с защищённой резервной копией. Если при восстановлении поменять `SESSION_SECRET`, ранее созданные сессии станут недействительны.

## Возврат предыдущего выпуска

В SSH-сеансе, после проверки пути в `previous-release`:

```sh
sudo cat /opt/tksu-pl/shared/previous-release
# Подставьте показанный каталог вместо <previous-release>:
sudo docker compose -p tksu-pl --env-file <previous-release>/.env -f <previous-release>/compose.production.yaml up -d
curl --fail http://127.0.0.1:8095/ping
sudo ln -sfn <previous-release> /opt/tksu-pl/current
```

Откат бинарника сохраняет текущую базу. Если миграция несовместима со старой версией, сначала остановите `tksu-pl`, сохраните текущую базу отдельным snapshot и восстановите согласованную копию через SQLite Backup API. После восстановления владелец каталога/файлов данных должен быть `10001:10001`. Не восстанавливайте файл базы одновременно с работающим приложением.

## Вернуть домен прежнему проекту

Этот сценарий нужен только при решении снова запустить предыдущий сайт:

1. `sudo docker start integradar-web` и проверить `http://127.0.0.1:8082`.
2. В `/opt/botmost/shared/Caddyfile` заменить только upstream `tksu-pl:8080` на `integradar-web:4173`. Не заменять весь файл старым бэкапом, если после миграции появились другие изменения.
3. `sudo docker exec botmost-caddy-1 caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile`.
4. `sudo docker kill --signal=SIGUSR1 botmost-caddy-1`.
5. Проверить сайт, затем при необходимости остановить `tksu-pl`.

При переносе «Трат» на новый домен добавьте соответствующий блок Caddy и обновите `PUBLIC_URL` в `/opt/tksu-pl/shared/.env`, сохранив `COOKIE_SECURE=true` и текущий секрет. Пересоздайте только контейнер приложения через Compose. DNS нового домена должен указывать на этот сервер.
