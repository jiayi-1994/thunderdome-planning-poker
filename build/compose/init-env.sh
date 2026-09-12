#!/bin/sh
set -eu

cd "$(dirname "$0")"
if [ -e .env ] || [ -L .env ]; then
  printf '%s\n' '.env already exists; existing credentials were preserved.'
  exit 0
fi

command -v openssl >/dev/null 2>&1 || {
  printf '%s\n' 'openssl is required; install it or fill .env.example manually.' >&2
  exit 1
}
db_password=$(openssl rand -hex 32)
cookie_key=$(openssl rand -hex 32)
aes_key=$(openssl rand -hex 32)

umask 077
# Prevent an overlapping invocation from replacing an existing credentials file.
set -C
sed \
  -e "s/^DB_PASSWORD=.*/DB_PASSWORD=$db_password/" \
  -e "s/^COOKIE_HASHKEY=.*/COOKIE_HASHKEY=$cookie_key/" \
  -e "s/^CONFIG_AES_HASHKEY=.*/CONFIG_AES_HASHKEY=$aes_key/" \
  .env.example > .env
printf '%s\n' 'Created .env with random credentials. Set APP_DOMAIN, then run docker compose up -d.'
