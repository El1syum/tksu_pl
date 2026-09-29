#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ -e .env ]; then echo '.env already exists; edit it manually.' >&2; exit 1; fi
umask 077
secret=$(openssl rand -hex 32)
sed "s/^SESSION_SECRET=$/SESSION_SECRET=$secret/" .env.example > .env
echo 'Created .env. Run: go run .'

