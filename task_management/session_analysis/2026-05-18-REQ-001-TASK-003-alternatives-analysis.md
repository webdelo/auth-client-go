# Alternatives Analysis — REQ-001-TASK-003

**Date:** 2026-05-18
**Context:** В ходе генерации PRP для TASK-003 обнаружена implicit cross-task зависимость: TASK-003 ссылается на типы и функции, которые в текущей декомпозиции принадлежат TASK-005.

---

## Проблема

TASK-003 (`ValidateAccessToken + HTTPMiddleware + UnaryInterceptor`) требует:

| Символ | Где используется | Где определяется по текущей декомпозиции |
|--------|------------------|------------------------------------------|
| `User` struct | возврат `ValidateAccessToken`, кэш, ctx-payload | TASK-005 (`lib/context.go`) |
| `Role` тип + константы | поле `User.Role`, маппинг `Role(resp.Role)` | TASK-005 (`lib/context.go`) |
| `WithUser(ctx, user)` | HTTPMiddleware step 8, UnaryInterceptor step 6 | TASK-005 |
| `UserFromContext` | не используется в TASK-003 (для consumer), но рядом | TASK-005 |
| `userCtxKey` | unexported (EC-0056) | TASK-005 |

TASK-003 и TASK-005 — siblings (оба `depends_on: REQ-001-TASK-002`), не sequential.

`Cache` интерфейс уже объявлен в `lib/client.go` (TASK-002 PRP) с `user any` как заглушкой, потому что `User` ещё не существовал — это документировано как «refine в TASK-003».

---

## Альтернативы

### Option A: TASK-003 берёт всё — выбрано

TASK-003 создаёт `lib/context.go` целиком:
- `Role string` + `RoleUnknown/RoleUser/RoleAdmin`
- `User struct { UUID string; Role Role; Email string }`
- Методы `IsZero / IsAdmin / HasRole`
- `userCtxKey struct{}` (unexported)
- `WithUser(ctx, User) context.Context`
- `UserFromContext(ctx) (User, bool)`

TASK-005 теряет пункт 1 (Context helpers) и сводится к `cookies.go` + `helpers.go`.

**Pros:**
- Один контракт `User`/`Role` появляется один раз и в правильном файле.
- Никаких stub-файлов, никакой миграции типов между файлами в TASK-005.
- TASK-003 self-contained — может тестироваться целиком (включая context-payload).
- `Cache` интерфейс в `client.go` финализируется один раз (`any` → `User`).

**Cons:**
- Нужно обновить TASK-005 spec — вычеркнуть пункт 1 из WHAT, скорректировать DOD.
- TASK-003 становится крупнее (но всё ещё в рамках сложности 7/10 из таблицы декомпозиции).

### Option B: TASK-003 минимум, TASK-005 переносит

TASK-003 кладёт `User`/`Role` в `lib/validate.go` или временный `lib/types.go`, плюс `WithUser`/`UserFromContext` приватно. TASK-005 переносит всё в `lib/context.go` и добавляет методы.

**Pros:** скоупы TASK-005 не трогаются.

**Cons:**
- File-shuffle между тасками — лишний шум в diff/PR.
- Compilation churn: при выполнении TASK-005 нужно одновременно удалить из validate.go и добавить в context.go.
- Сложнее ревьюить — типы вначале «не там».

### Option C: Переехать в TASK-002

Расширить TASK-002 PRP — добавить `lib/context.go` туда.

**Pros:** все типы появляются в первой реализующей таске.

**Cons:**
- TASK-002 PRP уже `status: prp_ready`, согласован, score 9/10. Редактирование = перепрогон approval.
- TASK-002 «Core Client + Errors + Logger» теряет фокус — становится «всё что нужно для следующих TASK».
- Cache интерфейс в TASK-002 PRP уже принят с `user any` именно потому что User ещё нет.

---

## Решение

**Option A — TASK-003 берёт всё.**

Обоснование:
1. Один проход, один файл, минимум churn.
2. TASK-005 spec обновляется одним вычеркиванием пункта 1 — это редакция, а не структурное изменение.
3. TASK-003 сохраняет естественное содержимое: типы, которые TASK-003 первой использует.
4. Cache интерфейс в `client.go` финализируется в один шаг (часть «Updates to client.go» в TASK-003).

Зафиксировано в PRP REQ-001-TASK-003: новый раздел Implementation Tasks включает создание `lib/context.go` с полным набором типов и context-хелперов. TASK-005 spec потребуется править отдельно (вычеркнуть пункт 1, обновить DOD) — это указано в Integration secciónе PRP как post-PRP follow-up.

---

## Ссылки

- TASK-003 spec: `task_management/tasks/REQ-001-TASK-003.md`
- TASK-005 spec: `task_management/tasks/REQ-001-TASK-005.md`
- TASK-002 PRP (Cache `any` placeholder): `task_management/plans/REQ-001-TASK-002-PRP.md:638-645`
- Code research: `.memory_bank/agent_task_results/2026-05-15_code-research_REQ-001-TASK-003.md` §4 G1
