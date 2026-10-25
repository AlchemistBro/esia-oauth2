# Конфигурация

> [К оглавлению](./index.md)

Сервис читает JSON-файл, переданный флагом `-config`. Все поля обязательны;
неподдерживаемый `id_token_algorithm` останавливает запуск.

Создайте локальный файл из безопасного шаблона:

```bash
cp config.example.json config.json
chmod 600 config.json
```

## Поля

| Поле | Формат и назначение |
|---|---|
| `redirect_uri` | Полный HTTPS callback URL, в точности зарегистрированный для ИС |
| `mnemonic` | Мнемоника ИС, используемая как OAuth client identifier и ожидаемый `aud` |
| `esia_uri` | Базовый URL контура ЕСИА: `https://esia-portal1.test.gosuslugi.ru` (тест) или `https://esia.gosuslugi.ru` |
| `csp_test_path` | Путь к `csptest` или к обёртке `deploy/csptest-wrapper.sh`, если на контейнере ключа есть PIN |
| `csp_container` | Имя контейнера ключа, как его видит CryptoPro (содержимое `name.key`) |
| `cert_hash` | Хеш ГОСТ Р 34.11-2012 (256) клиентского сертификата в HEX: `cpverify -mk cert.cer -alg GR3411_2012_256` |
| `http_listen_addr` | Внутренний адрес HTTP listener, например `:8000` |
| `esia_response_cert_path` | Путь к доверенному публичному сертификату TESIA |
| `id_token_algorithm` | `GOST3410_2012_256` или `RS256` |
| `id_token_issuer` | Точное ожидаемое значение claim `iss`, включая завершающий slash, если он предусмотрен контуром |
| `oauth.scopes` | Scopes запроса. Должен содержать `openid`; допустимы `openid`, `fullname`, `email` |
| `consent.permissions` | Необязательный запрос согласия на доступ к данным (ЕСИА v2). Если задан, `oauth.scopes` должен быть ровно `["openid"]` |

### Запрос ФИО и email через согласие

ЕСИА выдаёт ФИО и email по согласию пользователя. В `consent.permissions`
перечисляются запрашиваемые данные; сервис проверяет, что у каждого
разрешения заданы `sysname`, `expire > 0`, `actions`, `purposes` и `scopes`.
Если среди scopes есть `fullname` или `email`, после проверки ID token сервис
сам сходит в REST API ЕСИА и вернёт эти данные.

Без блока `consent` сервис выполняет обычный OpenID-вход и подтверждает
только личность пользователя (OID из `sub`).

## Безопасный пример

```json
{
  "redirect_uri": "https://service.example.com/auth/esia/callback",
  "mnemonic": "DEMO_CLIENT",
  "esia_uri": "https://esia-portal1.test.gosuslugi.ru",
  "csp_test_path": "/usr/local/bin/csptest-wrapper",
  "csp_container": "00000000-0000-0000-0000-000000000000",
  "cert_hash": "0000000000000000000000000000000000000000000000000000000000000000",
  "http_listen_addr": ":8000",
  "esia_response_cert_path": "/app/certs/tesia-response.cer",
  "id_token_algorithm": "GOST3410_2012_256",
  "id_token_issuer": "http://esia-portal1.test.gosuslugi.ru/",
  "oauth": {
    "scopes": ["openid"]
  },
  "consent": {
    "permissions": [
      {
        "sysname": "APIPGU",
        "expire": 525600,
        "actions": [{ "sysname": "ALL_ACTIONS_TO_DATA" }],
        "purposes": [{ "sysname": "APIPGU" }],
        "scopes": [{ "sysname": "fullname" }, { "sysname": "email" }]
      }
    ]
  }
}
```

Этот пример не является готовой конфигурацией и не содержит действительных
реквизитов. Он проходит валидацию: это проверяет тест
`TestExampleConfigLoads`.

## Что не следует публиковать

`config.json` намеренно исключён из Git. Не добавляйте реальный mnemonic,
redirect URI, certificate fingerprint, container identifier или внутренние
адреса даже в том случае, если конкретное поле не является секретом само по
себе.
