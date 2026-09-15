# Безопасность

> [К оглавлению](./index.md)

Этот документ описывает существующие механизмы и известные ограничения. Он не
является обещанием абсолютной безопасности или заменой аудита окружения.

## Реализованные проверки

- `state` и browser identifier создаются через `crypto/rand`;
- запись `state` имеет TTL 10 минут и поглощается только один раз;
- server-side transaction связывается с browser cookie;
- cookie использует `Secure`, `HttpOnly`, `SameSite=Lax` и префикс
  `__Host-`;
- OAuth endpoints принимают только GET;
- ID token должен использовать настроенный фиксированный `alg`;
- `typ` допускается отсутствующим или равным `JWT`;
- заголовок ID token обязан содержать `sbt=id`;
- подпись проверяется через CryptoPro GOST или RSA public key;
- проверяются `iss`, `aud`, `sub`, `exp`, `nbf` и `iat`;
- HTTP client и server имеют ограниченные timeouts;
- ответы OAuth flow помечаются `Cache-Control: no-store` и
  `Referrer-Policy: no-referrer`.

## Работа с secrets

Private key container не должен храниться в image. Реальные configuration,
certificates, key containers, PIN files и HAR исключены из Git и Docker build
context. В runtime их следует передавать отдельными mounts с минимальными
правами.

Callback URL содержит чувствительные одноразовые параметры. Не включайте query
string callback-запросов в публичные логи, bug reports или monitoring labels.

## Threat model и ограничения

- state store находится в памяти одного процесса;
- горизонтальное масштабирование без shared store нарушит проверку callback;
- restart сбрасывает незавершённые login transactions;
- защита host, Docker daemon и CryptoPro installation находится вне сервиса;
- TLS termination выполняет reverse proxy;
- внутренний порт 8000 не должен публиковаться напрямую на host;
- сервис подтверждает OAuth flow, но не создаёт application session;
- безопасность trust model зависит от корректности TESIA certificate;
- срок и отзыв клиентского сертификата контролирует оператор.

## Сообщение об уязвимости

Следуйте [политике безопасности](../../SECURITY.md). Не прикладывайте
credentials, tokens, cookies, key material или реальные конфигурации.
