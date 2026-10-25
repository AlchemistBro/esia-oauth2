# Устранение неполадок

> [К оглавлению](./index.md)

Диагностируйте ошибки без публикации authorization code, `state`, tokens,
cookies, PIN, fingerprints или реальных URL.

## ESIA authorization request возвращает internal error

`go-api-epgu v0.5.0` при `permissions=nil` может добавить параметр
`permissions=bnVsbA`, где `bnVsbA` — Base64URL-представление JSON
`null`.

Функция `withoutNilPermissions` удаляет только это точное сочетание. Если
`permissions` отсутствует или содержит настоящий запрос согласия, URL не
меняется. Повторяющийся параметр `permissions` отклоняется.

## ЕСИА отвечает `invalid_scope`

ИС не имеет доступа к запрошенным данным. Проверьте, что scopes из
`consent.permissions` (`fullname`, `email`) разрешены для вашей ИС на
технологическом портале, или временно уберите блок `consent`, чтобы проверить
обычный OpenID-вход.

## Подпись не работает в Docker, а на хосте работает

Почти всегда это права или PIN:

- при `cap_drop: ALL` файлы ключа и PIN должны принадлежать root, см.
  [права на файлы ключа](./cryptopro.md#права-на-файлы-ключа);
- если на контейнере есть PIN, `csp_test_path` должен указывать на
  `csptest-wrapper`, а файл PIN не должен заканчиваться переводом строки;
- пробный период CryptoPro закончился: проверьте `cpconfig -license -view`.

## CryptoPro container not found

Проверьте, что:

- container смонтирован в runtime;
- пользователь процесса имеет необходимые права;
- `csp_container` соответствует имени или пути, известному этой установке
  CryptoPro;
- архитектура и версия CSP соответствуют container image.

Не выводите содержимое container или PIN в логи.

## Certificate hash mismatch

`cert_hash` должен относиться к клиентскому сертификату, связанному с private
key container. Не путайте его с TESIA response certificate или TLS
certificate reverse proxy.

## Callback state rejected

Возможные причины:

- callback открыт в другом браузере или без pre-auth cookie;
- прошло больше 10 минут;
- container перезапущен между login и callback;
- тот же callback уже был использован;
- запрос попал в другой replica без shared state store.

Начните flow заново с `/auth/esia/login`. Не пытайтесь повторно использовать
старый callback URL.

## Invalid ID token

Проверьте согласованность `id_token_algorithm`, `id_token_issuer`,
`mnemonic` и TESIA response certificate с выбранным контуром. Token будет
отклонён также при неверной подписи, audience, subject или временных
claims.

Не отключайте отдельные проверки для обхода ошибки. Сначала подтвердите
ожидаемые значения по документации выбранного контура.

## Container запускается, но OAuth flow не работает

Проверьте:

1. DNS и HTTPS-доступ контейнера к ESIA endpoints;
2. точное совпадение зарегистрированного `redirect_uri`;
3. доступность CryptoPro command и client key container;
4. routing `/auth/esia/login` и `/auth/esia/callback` через reverse proxy;
5. синхронизацию времени host;
6. безопасные application logs без callback query.

Сетевой доступ и успешный старт процесса сами по себе не подтверждают
CryptoPro signing, Token Exchange или ID token validation — для этого нужен
полный E2E.
