# CryptoPro и сертификаты

> [К оглавлению](./index.md)

В интеграции участвуют три независимых типа сертификатов и ключевого
материала. Их нельзя взаимозаменять.

## Клиентский сертификат

Client certificate идентифицирует информационную систему. Вместе с
соответствующим private key он используется CryptoPro CSP для подписи
исходящих OAuth requests.

В конфигурации с ним связаны:

- `cert_hash` — отпечаток сертификата;
- `csp_container` — container с private key;
- `csp_test_path` — команда, через которую сервис вызывает CryptoPro.

## Private key container

Container содержит закрытый ключ клиентского сертификата. Он является
секретом, не должен попадать в Git или Docker image и передаётся runtime
отдельным mount. Права на container должны позволять работу только
пользователю процесса.

Если CryptoPro требует PIN, способ его неинтерактивной передачи настраивается
оператором вне репозитория. PIN нельзя включать в `config.json`, Dockerfile,
Compose-файл или command line, попадающую в process list.

## Публичный сертификат TESIA

TESIA response certificate содержит только публичный ключ ЕСИА. Сервис
использует его для проверки подписи входящего ID token. Путь задаётся полем
`esia_response_cert_path`.

Даже публичный сертификат следует получать из доверенного источника и
заменять контролируемо: подмена этого файла изменяет trust anchor.

## TLS-сертификат

TLS certificate обслуживает HTTPS на reverse proxy. Он не подписывает OAuth
request и не проверяет ID token. В рекомендуемой схеме TLS termination
выполняет nginx, а callback service слушает внутренний HTTP-порт.

## Что отсутствует в репозитории

- лицензированные CryptoPro installers и packages;
- serial license;
- private key containers;
- PIN/password;
- реальные fingerprints и container names;
- сертификаты конкретного окружения.

Точные команды установки и импорта зависят от версии, лицензии и документации
поставщика CryptoPro, поэтому проект их не подменяет универсальной инструкцией.
