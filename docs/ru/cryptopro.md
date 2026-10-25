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

## PIN контейнера

Библиотека `go-api-epgu` вызывает `csptest -keys -sign` без пароля, поэтому
контейнер с PIN она подписать не может. Для этого в репозитории есть обёртка
[`deploy/csptest-wrapper.sh`](../../deploy/csptest-wrapper.sh):

- для команд подписи она добавляет `-password`, читая PIN из
  `/run/secrets/csp-pin`;
- проверку подписи ID token пропускает без изменений;
- если файл PIN пуст, вызывает `csptest` как есть.

Укажите обёртку в `csp_test_path`, а PIN положите в файл без перевода строки:

```bash
printf '%s' 'PIN' > secrets/csp-pin
```

PIN нельзя включать в `config.json`, Dockerfile или Compose-файл. Обёртка
передаёт его `csptest` аргументом только внутри изолированного контейнера, где
нет других процессов.

## Права на файлы ключа

В `compose.example.yaml` включён `cap_drop: ALL`. Без `CAP_DAC_OVERRIDE` root
внутри контейнера не может читать файлы, которые на хосте принадлежат другому
пользователю. Типичный симптом: контейнер не виден в
`csptest -keyset -enum_cont`. Решение:

```bash
sudo chown -R root:root secrets
sudo chmod 700 secrets/key-container
sudo chmod 600 secrets/key-container/* secrets/csp-pin
```

## Лицензия CryptoPro CSP

Без лицензии CSP работает 90 дней пробного периода с момента установки, то
есть со сборки Docker-образа. Когда срок истечёт, подпись перестанет работать.
Серверная лицензия передаётся при сборке:

```bash
docker compose -f compose.example.yaml build --build-arg CPRO_LICENSE=XXXXX-XXXXX-XXXXX-XXXXX-XXXXX
```

Проверить срок внутри работающего контейнера:

```bash
docker exec esia-callback /opt/cprocsp/sbin/amd64/cpconfig -license -view
```

## Проверка ключа и подписи

```bash
docker exec esia-callback /opt/cprocsp/bin/amd64/csptest -keyset -enum_cont -fqcn -verifyc
docker exec esia-callback sh -c 'echo test > /tmp/t && /usr/local/bin/csptest-wrapper \
  -keys -sign GOST12_256 -cont "<csp_container>" -keytype exchange -in /tmp/t -out /tmp/s; echo rc=$?'
```

`rc=0` означает, что ключ доступен, PIN верен и лицензия действует.

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

- лицензированные CryptoPro installers и packages (их нужно скачать с
  [cryptopro.ru](https://cryptopro.ru/products/csp/downloads) и положить в
  `deploy/cryptopro/`: `lsb-cprocsp-base`, `lsb-cprocsp-rdr-64`,
  `lsb-cprocsp-kc1-64`, `lsb-cprocsp-capilite-64`, `lsb-cprocsp-ca-certs`,
  `cprocsp-curl-64`);
- serial license;
- private key containers;
- PIN/password;
- реальные fingerprints и container names;
- сертификаты конкретного окружения.

Точные команды установки и импорта зависят от версии, лицензии и документации
поставщика CryptoPro, поэтому проект их не подменяет универсальной инструкцией.
