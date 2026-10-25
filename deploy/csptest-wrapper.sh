#!/bin/sh
# Обёртка над csptest: go-api-epgu вызывает подпись без PIN, поэтому PIN
# контейнера подставляется здесь из файла-секрета. Проверка подписи ID token
# проходит без изменений.
set -eu

CSPTEST="${CSPTEST:-/opt/cprocsp/bin/amd64/csptest}"
PIN_FILE="${CSP_PIN_FILE:-/run/secrets/csp-pin}"

case " $* " in
  *" -sign "*)
    if [ -s "$PIN_FILE" ]; then
      exec "$CSPTEST" "$@" -password "$(cat "$PIN_FILE")"
    fi
    ;;
esac

exec "$CSPTEST" "$@"
