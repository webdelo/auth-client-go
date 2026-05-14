# SQL Schema Design Rules

## DB-SCHEMA-001: No CASCADE in Foreign Keys

**Severity:** CRITICAL
**Applies to:** All SQL databases (MySQL, PostgreSQL, SQLite)

### Rule

**ЗАПРЕЩЕНО** использовать `ON DELETE CASCADE`, `ON UPDATE CASCADE` и любые другие каскадные действия в определениях внешних ключей.

Весь менеджмент связанных записей (удаление, обновление, архивирование) должен выполняться **исключительно на стороне приложения**.

### Why

1. **Непредсказуемые побочные эффекты** — каскадное удаление может затронуть записи в таблицах, о которых разработчик не думал при написании запроса
2. **Сложность отладки** — удаление одной записи неявно удаляет десятки/сотни записей в зависимых таблицах, что крайне сложно отследить
3. **Проблемы при миграциях** — добавление/изменение внешних ключей с CASCADE усложняет миграции и может привести к потере данных
4. **Нарушение принципа явности** — бизнес-логика удаления/обновления скрыта в DDL вместо явного описания в коде приложения
5. **Производительность** — каскадные операции выполняются в рамках одной транзакции БД, что может вызвать блокировки и тайм-ауты

### Bad (FORBIDDEN)

```sql
-- ON DELETE CASCADE
ALTER TABLE signal_approvers
    ADD CONSTRAINT signal_hash_fk
    FOREIGN KEY (signal_hash) REFERENCES signals(hash)
    ON DELETE CASCADE;

-- ON UPDATE CASCADE
ALTER TABLE orders
    ADD CONSTRAINT user_fk
    FOREIGN KEY (user_id) REFERENCES users(id)
    ON UPDATE CASCADE;

-- Any combination
CREATE TABLE order_items (
    id BIGINT PRIMARY KEY,
    order_id BIGINT,
    FOREIGN KEY (order_id) REFERENCES orders(id)
        ON DELETE CASCADE ON UPDATE CASCADE
);
```

### Good (REQUIRED)

```sql
-- Foreign key WITHOUT cascade actions (default RESTRICT behavior)
ALTER TABLE signal_approvers
    ADD CONSTRAINT signal_hash_fk
    FOREIGN KEY (signal_hash) REFERENCES signals(hash);

-- Explicit RESTRICT is also acceptable
ALTER TABLE orders
    ADD CONSTRAINT user_fk
    FOREIGN KEY (user_id) REFERENCES users(id)
    ON DELETE RESTRICT ON UPDATE RESTRICT;

-- CREATE TABLE without cascade
CREATE TABLE order_items (
    id BIGINT PRIMARY KEY,
    order_id BIGINT,
    FOREIGN KEY (order_id) REFERENCES orders(id)
);
```

### Application-Side Deletion Pattern (Go)

```go
func (r *Repository) DeleteSignalWithDependencies(ctx context.Context, hash string) error {
    return r.db.Transaction(func(tx *gorm.DB) error {
        // 1. Delete dependent records explicitly
        if err := tx.Where("signal_hash = ?", hash).Delete(&SignalApprover{}).Error; err != nil {
            return fmt.Errorf("delete approvers: %w", err)
        }

        if err := tx.Where("signal_hash = ?", hash).Delete(&SignalKline{}).Error; err != nil {
            return fmt.Errorf("delete klines: %w", err)
        }

        // 2. Delete parent record
        if err := tx.Where("hash = ?", hash).Delete(&Signal{}).Error; err != nil {
            return fmt.Errorf("delete signal: %w", err)
        }

        return nil
    })
}
```

### Enforcement

- **Code review**: Any SQL with `CASCADE` must be rejected
- **ADR review**: Database schema proposals must not include cascade actions
- **Migration review**: New migrations must not introduce CASCADE constraints

### Applies To

- DDL statements (`CREATE TABLE`, `ALTER TABLE`)
- Migration files (`database/*_migrations/`)
- ADR database schema sections
- Any SQL schema design documentation
