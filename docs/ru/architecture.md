# Архитектура

> [К оглавлению](./index.md)

Сервис предоставляет два HTTP endpoint и хранит незавершённые
авторизационные транзакции в памяти одного процесса.

## Схемы C4

Исходники в формате draw.io лежат в [`docs/diagrams`](../diagrams); их можно
открыть в [diagrams.net](https://app.diagrams.net) и править.

### Уровень 1. Контекст

![C4: контекст](../assets/c4-1-context.svg)

### Уровень 2. Контейнеры

![C4: контейнеры](../assets/c4-2-containers.svg)

### Уровень 3. Компоненты

![C4: компоненты](../assets/c4-3-components.svg)

### Развёртывание

![C4: развёртывание](../assets/c4-4-deployment.svg)

## Последовательность

![UML: последовательность входа](../assets/sequence-login.svg)

## `GET /auth/esia/login`

Сервис получает или создаёт случайный browser identifier, формирует
authorization request через `go-api-epgu` и CryptoPro, извлекает созданный
`state` и сохраняет его с TTL 10 минут. Browser identifier записывается в
cookie `__Host-esia_pre_auth` с атрибутами `Secure`, `HttpOnly` и
`SameSite=Lax`. Затем браузер получает HTTP 302 на authorization endpoint
ЕСИА.

Перед redirect функция `withoutNilPermissions` удаляет только точное значение
`permissions=bnVsbA`, создаваемое зависимостью для JSON `null`. Настоящий
запрос согласия из `consent.permissions` передаётся без изменений, а
повторяющийся параметр `permissions` приводит к ошибке.

## Почему используются и map, и cookie

Они проверяют разные свойства одной транзакции:

- запись в server-side map подтверждает, что `state` был выпущен этим
  процессом, ещё не истёк и не использовался;
- cookie связывает эту транзакцию с браузером, который начал flow.

Cookie не заменяет server-side запись, а запись не подтверждает браузер без
cookie.

## `GET /auth/esia/callback`

Сервис разбирает authorization response, проверяет cookie и атомарно удаляет
соответствующий `state`. Повторное использование того же `state`
отклоняется. После этого authorization code обменивается на access token и ID
token.

ID token разбирается как JWT. До дорогостоящей проверки подписи проверяются
ожидаемый `alg` и допустимый `typ`. Затем сервис проверяет ГОСТ- или
RSA-подпись, `iss`, `aud`, наличие `sub` и временные claims `exp`, `nbf`,
`iat` с допустимым clock skew 60 секунд.

ЕСИА передаёт ГОСТ-подпись в JWT в порядке байтов big-endian, а `csptest`
ожидает обратный порядок. Сервис разворачивает подпись перед проверкой;
без этого любой корректный токен выглядит поддельным.

Если в `consent.permissions` запрошены `fullname` или `email`, сервис берёт
OID пользователя из `sub` проверенного токена и запрашивает REST API ЕСИА:
`/rs/prns/{oid}` для ФИО и `/rs/prns/{oid}/ctts` для email. Access token при
этом не покидает сервер.

## Жизненный цикл

State store находится в памяти:

- deployment рассчитан на один instance;
- restart сбрасывает незавершённые login transactions;
- после restart пользователь должен начать авторизацию заново;
- для нескольких replicas требуется общий атомарный store с TTL и операцией
  consume-once.
