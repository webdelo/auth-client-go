name: "DevOps Task template v1 - Infrastructure & Deployment Task"
description: |

# Шаблон DevOps задачи (DEVOPS-TASK)

Задача описывает конкретную инфраструктурную или deployment работу.

## Заголовок
**[Название DevOps задачи, краткое описание инфраструктурного изменения]**

---

## REQ
**Родительский реквест**: [REQ-XXX: Название реквеста](./REQ-XXX.md)

---

## WHY

Описание контекста DevOps задачи и причин, по которым данная инфраструктурная задача должна быть решена.
Как эта задача влияет на deployment, масштабирование, безопасность или мониторинг системы.

---

## WHAT

Пошаговая подробная инструкция, включающая исчерпывающий перечень действий, которые должны быть выполнены
для реализации инфраструктурного решения.

### **Описание шагов реализации**

- _[Описать необходимые к выполнению инфраструктурные действия]_
- _[Конфигурации, которые нужно создать/изменить]_
- _[Deployment стратегия]_

### **Инфраструктурные компоненты**

#### Containerization (если применимо)
- Base image и версия
- Multi-stage build требования
- Security hardening требования
- Health check конфигурация
- Resource limits

#### Kubernetes Resources (если применимо)
- Deployment/StatefulSet спецификации
- Service и Ingress конфигурации
- ConfigMaps и Secrets
- PersistentVolumeClaims
- HorizontalPodAutoscaler настройки
- Network Policies

#### CI/CD Pipeline (если применимо)
- Build steps
- Test stages
- Security scanning
- Deployment strategy (rolling, blue-green, canary)
- Rollback procedure

#### Monitoring & Logging (если применимо)
- Metrics to expose
- Log aggregation setup
- Alerting rules
- Dashboards requirements

### **Целевая среда**

- [ ] Development
- [ ] Staging
- [ ] Production

### **Зависимости**

- _[Внешние сервисы, volumes, secrets, configmaps]_
- _[Инфраструктурные требования (load balancer, storage class)]_

### **Конфигурационные файлы**

_[Список конфигурационных файлов, которые будут созданы/изменены]_

- `Dockerfile`
- `docker-compose.yml`
- `k8s/deployment.yaml`
- `k8s/service.yaml`
- `.github/workflows/deploy.yml`
- и т.д.

### **Ссылки на документацию**

- _[Ссылка на официальную документацию Docker]_
- _[Ссылка на официальную документацию Kubernetes]_
- _[Ссылка на документацию используемых инструментов]_

---

## DOD (Definition of Done)

### Критерии готовности

- [ ] Конфигурационные файлы созданы и проверены
- [ ] Security scan пройден без critical/high уязвимостей
- [ ] Resource limits и requests определены
- [ ] Health checks (liveness, readiness) настроены
- [ ] Secrets управляются безопасно (не хардкоднуты)
- [ ] Logging настроен на stdout/stderr
- [ ] Metrics экспортируются корректно
- [ ] Deployment успешно прошел в целевой среде
- [ ] Rollback процедура документирована и протестирована
- [ ] Monitoring dashboards обновлены
- [ ] Alerts настроены (если требуется)

### Обязательные проверки

- [ ] Docker image builds successfully
- [ ] Container runs as non-root user
- [ ] All secrets mounted from external sources
- [ ] Health endpoints respond correctly
- [ ] Resource usage within defined limits
- [ ] Logs structured and accessible
- [ ] Metrics available in monitoring system
- [ ] Deployment can be rolled back
- [ ] Documentation updated (README, runbooks)

### Тестовые сценарии

- [ ] _[Happy path deployment]_
- [ ] _[Rollback scenario]_
- [ ] _[Health check failure handling]_
- [ ] _[Resource limit breach behavior]_
- [ ] _[Configuration change application]_
