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
| `esia_uri` | Базовый URL выбранного контура ЕСИА без данных пользователя |
| `csp_test_path` | Абсолютный путь к `csptest` или совместимой локальной обёртке |
| `csp_container` | Идентификатор client private key container в формате локальной установки CSP |
| `cert_hash` | Отпечаток клиентского сертификата для подписи исходящих запросов |
| `http_listen_addr` | Внутренний адрес HTTP listener, например `:8000` |
| `esia_response_cert_path` | Путь к доверенному публичному сертификату TESIA |
| `id_token_algorithm` | `GOST3410_2012_256` или `RS256` |
| `id_token_issuer` | Точное ожидаемое значение claim `iss`, включая завершающий slash, если он предусмотрен контуром |

## Безопасный пример

```json
{
  "redirect_uri": "https://service.example.com/auth/esia/callback",
  "mnemonic": "DEMO_CLIENT",
  "esia_uri": "https://esia.example.test",
  "csp_test_path": "/opt/cprocsp/bin/amd64/csptest",
  "csp_container": "example-client-key-container",
  "cert_hash": "0000000000000000000000000000000000000000",
  "http_listen_addr": ":8000",
  "esia_response_cert_path": "/app/certs/tesia-response.cer",
  "id_token_algorithm": "GOST3410_2012_256",
  "id_token_issuer": "https://esia.example.test/"
}
```

Этот пример не является готовой конфигурацией и не содержит действительных
реквизитов.

## Что не следует публиковать

`config.json` намеренно исключён из Git. Не добавляйте реальный mnemonic,
redirect URI, certificate fingerprint, container identifier или внутренние
адреса даже в том случае, если конкретное поле не является секретом само по
себе.
