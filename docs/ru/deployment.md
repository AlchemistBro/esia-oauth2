# Развёртывание

> [К оглавлению](./index.md)

Рекомендуемая схема не публикует HTTP listener сервиса на host:

```text
Internet
   │
   ▼
nginx :443
   │  Docker network
   ▼
esia-callback :8000
```

nginx завершает TLS и проксирует только нужный route во внутреннюю Docker
network. Значение `redirect_uri` должно точно соответствовать публичному
HTTPS callback URL, зарегистрированному для ИС.

## Подготовка файлов

Перед сборкой оператор локально создаёт:

```text
deploy/cryptopro/           # licensed .deb packages
certs/tesia-response.cer    # trusted TESIA public certificate
secrets/client-key-container/
config.json
```

Эти пути исключены из Git. Не заменяйте placeholders в
`compose.example.yaml` реальными значениями перед commit.

## Docker Compose

Проверьте mounts и запустите:

```bash
docker compose -f compose.example.yaml build
docker compose -f compose.example.yaml up -d
```

`compose.example.yaml` использует `expose`, а не `ports`. Контейнер
доступен другим участникам network `esia-backend`, но порт 8000 не
публикуется на host.

## Пример nginx

Ниже только нейтральный фрагмент routing. TLS certificate, logging и общие
security headers настраиваются отдельно.

```nginx
upstream esia_callback {
    server esia-callback:8000;
}

server {
    listen 443 ssl;
    server_name service.example.com;

    location /auth/esia/ {
        proxy_pass http://esia_callback;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

Если nginx находится в другом Compose project, подключите его к
`esia-backend`. Не направляйте внешний трафик напрямую на container IP.

## Проверка после запуска

```bash
docker compose -f compose.example.yaml ps
docker compose -f compose.example.yaml logs esia-callback
```

Затем начните ручной flow через
`https://service.example.com/auth/esia/login`. Не копируйте callback query,
tokens или cookies в диагностические материалы.

## Обновление

Собирайте image с отдельным immutable tag, сохраняйте предыдущий проверенный
image для rollback и меняйте только callback service. Перед production rollout
проверьте тесты, mounts, network, отсутствие опубликованного порта и полный
ручной E2E.
