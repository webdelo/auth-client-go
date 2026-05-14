#!/usr/bin/env python3
"""
Request Status Refresh Script
Актуализирует статус всех задач реквеста на основе реальных файлов в проекте.
Автоматически обнаруживает задачи, находит связанные файлы и обновляет tasks_list.yaml.
"""

import os
import re
import sys
import yaml
from pathlib import Path
from datetime import datetime
from typing import Dict, List, Optional, Tuple
from dataclasses import dataclass, field
from collections import defaultdict


@dataclass
class DuplicateTaskError:
    """Информация о дублирующейся задаче"""
    task_id: str
    sections: List[str]  # Секции, в которых найден дубликат
    line_hints: List[str]  # Подсказки для поиска в файле


@dataclass
class TaskData:
    """Данные задачи, собранные из файловой системы"""
    task_id: str
    req_id: str
    task_file: str
    jira_id: Optional[str] = None
    prp_file: Optional[str] = None
    result_report: Optional[str] = None
    code_review_files: List[str] = field(default_factory=list)
    bug_report_files: List[str] = field(default_factory=list)
    status: str = 'pending'
    description: str = ''
    branch: str = ''
    priority: str = 'medium'
    depends_on: List[str] = field(default_factory=list)


class ReqRefreshService:
    """Сервис для актуализации статусов задач"""

    def __init__(self, project_root: Path):
        self.project_root = project_root
        self.tasks_list_path = project_root / ".memory_bank" / "tasks_list.yaml"
        self.tasks_dir = project_root / "task_management" / "tasks"
        self.plans_dir = project_root / "task_management" / "plans"
        self.results_dir = project_root / "task_management" / "results"
        self.reviews_dir = project_root / "task_management" / "reviews"
        self.bug_reports_dir = project_root / "task_management" / "bug_reports"
        self.agent_results_dir = project_root / ".memory_bank" / "agent_task_results"

    def parse_req_id(self, req_input: str) -> str:
        """Извлечь REQ ID из различных форматов ввода"""
        # Если это путь к файлу
        if '/' in req_input or req_input.endswith('.md'):
            match = re.search(r'(REQ-\d+)', req_input)
            if match:
                return match.group(1)

        # Если это просто REQ-XXX или REQ-XXX-TASK-YYY
        match = re.search(r'(REQ-\d+)', req_input.upper())
        if match:
            return match.group(1)

        # Если это просто номер
        if req_input.isdigit():
            return f"REQ-{req_input.zfill(3)}"

        raise ValueError(f"Cannot parse REQ ID from: {req_input}")

    def scan_filesystem_for_tasks(self, req_id: str) -> List[TaskData]:
        """
        Сканировать файловую систему и собрать информацию о всех задачах для реквеста.
        Возвращает список TaskData с полной информацией о найденных файлах.

        Jira ID наследуется с уровня REQ: если REQ файл содержит **Jira**: PROJ-XX,
        все дочерние задачи получают этот jira_id.
        """
        tasks = []

        # Сканируем директорию с задачами
        if not self.tasks_dir.exists():
            return tasks

        # Извлекаем Jira ID из REQ файла (наследуется всеми задачами)
        req_jira_id = self._extract_jira_id_from_req_file(req_id)

        pattern = f"{req_id}-TASK-*.md"
        for task_file in self.tasks_dir.glob(pattern):
            task_id = self._extract_task_id_from_filename(task_file.name)
            if not task_id:
                continue

            # Создаем объект задачи с jira_id наследованным от REQ
            task_data = TaskData(
                task_id=task_id,
                req_id=req_id,
                task_file=str(task_file.relative_to(self.project_root)),
                jira_id=req_jira_id
            )

            # Читаем описание из файла задачи
            task_data.description = self._extract_description_from_file(task_file)

            # Ищем все связанные файлы
            self._find_prp_file(task_data)
            self._find_result_file(task_data)
            self._find_code_review_files(task_data)
            self._find_bug_report_files(task_data)

            # Получаем lifecycle из существующих данных (если есть)
            lifecycle = self._get_existing_lifecycle(task_data.task_id)

            # Определяем статус на основе найденных файлов и lifecycle
            task_data.status = self._determine_status(task_data, lifecycle)

            # Извлекаем метаданные из файла задачи (может переопределить jira_id)
            self._extract_metadata_from_task_file(task_data, task_file)

            tasks.append(task_data)

        return sorted(tasks, key=lambda t: t.task_id)

    def _extract_task_id_from_filename(self, filename: str) -> Optional[str]:
        """Извлечь task_id из имени файла"""
        match = re.match(r'(REQ-\d+-TASK-\d+)\.md$', filename)
        return match.group(1) if match else None

    def _extract_description_from_file(self, file_path: Path) -> str:
        """Извлечь описание из первого заголовка файла задачи"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                for line in f:
                    line = line.strip()
                    if line.startswith('# '):
                        # Убираем "# " и возможные markdown символы
                        desc = line[2:].strip()
                        # Убираем task_id если он есть в начале
                        desc = re.sub(r'^REQ-\d+-TASK-\d+:\s*', '', desc)
                        return desc
        except Exception:
            pass
        return ''

    def _extract_jira_id_from_req_file(self, req_id: str) -> Optional[str]:
        """Извлечь Jira ID из файла реквеста (REQ).

        Jira ID задаётся на уровне REQ и наследуется всеми дочерними задачами.
        Ищет паттерн **Jira**: PROJ-123 в файле реквеста.
        """
        req_file = self.tasks_dir / f"{req_id}.md"
        if not req_file.exists():
            return None

        try:
            with open(req_file, 'r', encoding='utf-8') as f:
                content = f.read()

            # Match **Jira**: PROJ-123 (any letter prefix + digits)
            jira_match = re.search(
                r'\*\*Jira\*\*:\s*[_`]?([A-Z]+-\d+)[_`]?',
                content, re.IGNORECASE
            )
            if jira_match:
                return jira_match.group(1).upper()
        except Exception:
            pass

        return None

    def _extract_metadata_from_task_file(self, task_data: TaskData, file_path: Path):
        """Извлечь метаданные (branch, priority, depends_on, jira_id) из файла задачи"""
        try:
            with open(file_path, 'r', encoding='utf-8') as f:
                content = f.read()

                # Ищем branch
                branch_match = re.search(r'[**]*Branch[**]*:\s*`?([^\s`\n]+)`?', content, re.IGNORECASE)
                if branch_match:
                    task_data.branch = branch_match.group(1)

                # Ищем priority
                priority_match = re.search(r'[**]*Priority[**]*:\s*(\w+)', content, re.IGNORECASE)
                if priority_match:
                    priority = priority_match.group(1).lower()
                    if priority in ['critical', 'high', 'medium', 'low']:
                        task_data.priority = priority

                # Ищем depends_on
                depends_match = re.search(r'[**]*Depends on[**]*:\s*(.+)', content, re.IGNORECASE)
                if depends_match:
                    deps_str = depends_match.group(1)
                    # Извлекаем все REQ-XXX-TASK-YYY из строки
                    task_data.depends_on = re.findall(r'REQ-\d+-TASK-\d+', deps_str)

                # Ищем Jira ID в файле задачи (переопределяет REQ-уровневый jira_id)
                jira_match = re.search(
                    r'\*\*Jira\*\*:\s*[_`]?([A-Z]+-\d+)[_`]?',
                    content, re.IGNORECASE
                )
                if jira_match and not task_data.jira_id:
                    task_data.jira_id = jira_match.group(1).upper()

        except Exception:
            pass

    def _find_prp_file(self, task_data: TaskData):
        """Найти PRP файл для задачи"""
        prp_path = self.plans_dir / f"{task_data.task_id}-PRP.md"
        if prp_path.exists():
            task_data.prp_file = str(prp_path.relative_to(self.project_root))

    def _find_result_file(self, task_data: TaskData):
        """Найти result report файл для задачи"""
        result_path = self.results_dir / f"{task_data.task_id}-result-report.md"
        if result_path.exists():
            task_data.result_report = str(result_path.relative_to(self.project_root))

    def _find_code_review_files(self, task_data: TaskData):
        """Найти все code review файлы для задачи"""
        # Поиск в task_management/reviews/ — case-insensitive через перебор
        if self.reviews_dir.exists():
            for file in self.reviews_dir.glob(f"*{task_data.task_id}*.yaml"):
                if 'code-review' in file.name.lower():
                    rel_path = str(file.relative_to(self.project_root))
                    task_data.code_review_files.append(rel_path)

        # Поиск в .memory_bank/agent_task_results/
        if self.agent_results_dir.exists():
            for file in self.agent_results_dir.glob(f"CODE-REVIEW-{task_data.task_id}*.yaml"):
                rel_path = str(file.relative_to(self.project_root))
                task_data.code_review_files.append(rel_path)

    def _find_bug_report_files(self, task_data: TaskData):
        """Найти все bug report файлы для задачи"""
        # Основная директория: task_management/bug_reports/
        if self.bug_reports_dir.exists():
            for file in self.bug_reports_dir.glob(f"{task_data.task_id}*BUG-REPORT*.yaml"):
                rel_path = str(file.relative_to(self.project_root))
                if rel_path not in task_data.bug_report_files:
                    task_data.bug_report_files.append(rel_path)
        # Fallback: .memory_bank/agent_task_results/
        if self.agent_results_dir.exists():
            for file in self.agent_results_dir.glob(f"BUG-REPORT-{task_data.task_id}*.yaml"):
                rel_path = str(file.relative_to(self.project_root))
                if rel_path not in task_data.bug_report_files:
                    task_data.bug_report_files.append(rel_path)

    def _get_existing_lifecycle(self, task_id: str) -> dict:
        """Получить существующий lifecycle для задачи из tasks_list.yaml"""
        if not self.tasks_list_path.exists():
            return {}

        tasks_list = self.load_tasks_list()
        for section in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            for task in tasks_list.get(section, []):
                if task.get('task_id') == task_id:
                    return task.get('lifecycle', {})

        return {}

    def _read_code_review_verdict(self, code_review_files: List[str]) -> tuple:
        """Прочитать verdict из файлов code review на диске.

        Возвращает (verdict, critical_count).
        Приоритет вердиктов: CHANGES_REQUESTED > ALL_FIXED > APPROVED > APPROVED_WITH_COMMENTS.

        Поддерживает несколько форматов review файлов:
        - issues.critical (nested format)
        - top-level critical/warnings/suggestions (code-reviewer agent format)
        - review_fixes.status=ALL_FIXED overlay
        """
        verdicts = []
        total_critical = 0

        for file_path in code_review_files:
            full_path = self.project_root / file_path
            try:
                with open(full_path, 'r', encoding='utf-8') as f:
                    data = yaml.safe_load(f)
                if not data:
                    continue

                # Check review_fixes overlay first
                review_fixes = data.get('review_fixes')
                if review_fixes and str(review_fixes.get('status', '')).upper() == 'ALL_FIXED':
                    verdicts.append('ALL_FIXED')
                    continue

                verdict = data.get('verdict')
                if verdict:
                    verdicts.append(verdict)

                # Count critical issues - support multiple formats
                critical = []
                issues = data.get('issues', {})
                if isinstance(issues, dict):
                    critical = issues.get('critical', [])
                elif isinstance(issues, list):
                    critical = [i for i in issues if i.get('severity', '').upper() in ('CRITICAL', 'ERROR')]

                # Fallback: top-level critical key (code-reviewer agent format)
                if not critical and 'critical' in data:
                    critical = data.get('critical', [])

                if isinstance(critical, list):
                    total_critical += len(critical)
            except Exception:
                pass

        if not verdicts:
            return None, 0

        if 'CHANGES_REQUESTED' in verdicts:
            return 'CHANGES_REQUESTED', total_critical
        if 'ALL_FIXED' in verdicts:
            return 'ALL_FIXED', 0
        if 'APPROVED' in verdicts:
            return 'APPROVED', total_critical
        if 'APPROVED_WITH_COMMENTS' in verdicts:
            return 'APPROVED_WITH_COMMENTS', total_critical
        return verdicts[-1], total_critical

    def _determine_status(self, task_data: TaskData, lifecycle: dict = None) -> str:
        """
        Определить статус задачи на основе найденных файлов и lifecycle.

        Новая логика статусов (с учётом code review):
        - pending: задача создана, PRP нет
        - prp_ready: PRP готов, ожидает выполнения
        - in_progress: PRP выполняется (активная работа)
        - pending_review: выполнено, ожидает ревью
        - review_fixes: ревью есть, требуются исправления (CHANGES_REQUESTED)
        - completed: ревью пройдено (APPROVED/APPROVED_WITH_COMMENTS без critical/ALL_FIXED)

        Args:
            task_data: Данные задачи из файловой системы
            lifecycle: Lifecycle данные из tasks_list.yaml (если есть)
        """
        lifecycle = lifecycle or {}
        execution = lifecycle.get('execution', {})
        code_review = lifecycle.get('code_review', {})
        bug_fixing = lifecycle.get('bug_fixing', {})

        has_prp = bool(task_data.prp_file)
        has_result = bool(task_data.result_report)
        execution_completed = execution.get('completed', False)
        review_completed = code_review.get('completed', False)

        # 1. Нет PRP -> pending
        if not has_prp:
            return 'pending'

        # 2. PRP есть, result нет, execution не completed И ревью нет -> prp_ready
        #    Важно: если ревью уже проведено (review_completed=True), пропускаем этот шаг —
        #    задача была выполнена, даже если result_report не зарегистрирован в YAML.
        if not has_result and not execution_completed and not review_completed:
            # Проверяем наличие файлов ревью на диске перед выводом prp_ready
            if not task_data.code_review_files:
                return 'prp_ready'

        # 3. Если lifecycle не содержит данных о ревью, но файлы code review есть на диске —
        #    читаем verdict напрямую из файлов (актуально при первом запуске с пустым lifecycle)
        if not review_completed and task_data.code_review_files:
            file_verdict, file_critical = self._read_code_review_verdict(task_data.code_review_files)
            if file_verdict:
                review_completed = True
                code_review = {'completed': True, 'verdict': file_verdict, 'critical_issues': file_critical}

        # 4. Execution completed или result есть, ревью нет -> pending_review
        if not review_completed:
            return 'pending_review'

        # 5. Ревью проведено
        verdict = code_review.get('verdict')
        bugs_fixed = bug_fixing.get('bugs_fixed', False)

        # APPROVED или ALL_FIXED -> completed
        if verdict in ['APPROVED', 'ALL_FIXED']:
            return 'completed'

        # APPROVED_WITH_COMMENTS -> completed (reviewer approved)
        if verdict == 'APPROVED_WITH_COMMENTS':
            return 'completed'

        # CHANGES_REQUESTED
        if verdict == 'CHANGES_REQUESTED':
            if bugs_fixed:
                return 'completed'
            return 'review_fixes'

        # Fallback: ревью есть но verdict неизвестен -> pending_review
        return 'pending_review'

    def load_tasks_list(self) -> Dict:
        """Загрузить tasks_list.yaml"""
        if not self.tasks_list_path.exists():
            return self._create_empty_tasks_list()

        with open(self.tasks_list_path, 'r', encoding='utf-8') as f:
            return yaml.safe_load(f)

    def validate_unique_task_ids(self, tasks_list: Dict = None) -> List[DuplicateTaskError]:
        """
        Проверить уникальность task_id во всех секциях tasks_list.yaml.

        Возвращает список DuplicateTaskError с информацией о дубликатах.
        Пустой список означает, что дубликатов нет.
        """
        if tasks_list is None:
            tasks_list = self.load_tasks_list()

        # Собираем все task_id с указанием секций
        task_locations: Dict[str, List[Tuple[str, int]]] = defaultdict(list)

        for section in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            for idx, task in enumerate(tasks_list.get(section, [])):
                task_id = task.get('task_id')
                if task_id:
                    task_locations[task_id].append((section, idx))

        # Ищем дубликаты
        duplicates = []
        for task_id, locations in task_locations.items():
            if len(locations) > 1:
                sections = [loc[0] for loc in locations]
                line_hints = [f"{loc[0]}[{loc[1]}]" for loc in locations]
                duplicates.append(DuplicateTaskError(
                    task_id=task_id,
                    sections=sections,
                    line_hints=line_hints
                ))

        return duplicates

    def remove_duplicate_tasks(self, tasks_list: Dict) -> int:
        """
        Удалить дубликаты task_id из tasks_list, оставив только первое вхождение.

        Возвращает количество удалённых дубликатов.
        """
        seen_task_ids = set()
        removed_count = 0

        for section in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            if section not in tasks_list:
                continue

            original_tasks = tasks_list[section]
            unique_tasks = []

            for task in original_tasks:
                task_id = task.get('task_id')
                if task_id and task_id not in seen_task_ids:
                    seen_task_ids.add(task_id)
                    unique_tasks.append(task)
                elif task_id:
                    removed_count += 1

            tasks_list[section] = unique_tasks

        return removed_count

    def _create_empty_tasks_list(self) -> Dict:
        """Создать пустую структуру tasks_list"""
        return {
            'metadata': {
                'template_version': '1.0',
                'updated_at': datetime.now().strftime('%Y-%m-%dT%H:%M:%SZ'),
                'total_pending': 0,
                'total_active': 0,
                'total_completed': 0,
                'total_blocked': 0
            },
            'pending_tasks': [],
            'active_tasks': [],
            'completed_tasks': [],
            'blocked_tasks': [],
            'status_reference': {
                'feature_tasks': ['pending', 'prp_ready', 'in_progress', 'pending_review', 'reviewed', 'completed'],
                'bugfix_tasks': ['pending', 'analysis_complete', 'in_progress', 'pending_review', 'completed']
            }
        }

    def save_tasks_list(self, data: Dict):
        """Сохранить tasks_list.yaml"""
        # Убедимся что директория существует
        self.tasks_list_path.parent.mkdir(parents=True, exist_ok=True)

        with open(self.tasks_list_path, 'w', encoding='utf-8') as f:
            yaml.dump(data, f, allow_unicode=True, default_flow_style=False, sort_keys=False)

    def validate_yaml_files(self, tasks: List[TaskData]) -> List[Dict]:
        """
        Проверить, что все YAML файлы задач (code review, bug reports) открываются
        и парсятся без ошибок.

        Возвращает список ошибок: [{'file': path, 'task_id': id, 'error': str}]
        Пустой список означает, что все файлы валидны.
        """
        errors = []
        for task in tasks:
            for file_path in task.code_review_files + task.bug_report_files:
                full_path = self.project_root / file_path
                try:
                    with open(full_path, 'r', encoding='utf-8') as f:
                        yaml.safe_load(f)
                except (IOError, OSError) as e:
                    errors.append({
                        'file': file_path,
                        'task_id': task.task_id,
                        'error': f"Cannot open file: {e}"
                    })
                except yaml.YAMLError as e:
                    errors.append({
                        'file': file_path,
                        'task_id': task.task_id,
                        'error': f"YAML parse error: {e}"
                    })
        return errors

    def refresh_request(self, req_id: str, auto_fix_duplicates: bool = False) -> Dict:
        """
        Полная актуализация задач реквеста.

        Возвращает словарь с информацией об изменениях:
        - added: новые задачи
        - updated: обновленные задачи (изменен статус или файлы)
        - no_change: задачи без изменений
        - duplicates: список найденных дубликатов (DuplicateTaskError)
        - duplicates_removed: количество удалённых дубликатов (если auto_fix_duplicates=True)
        """
        # Сканируем файловую систему
        found_tasks = self.scan_filesystem_for_tasks(req_id)

        if not found_tasks:
            return {
                'added': [],
                'updated': [],
                'no_change': [],
                'duplicates': [],
                'error': f"No tasks found for {req_id}"
            }

        # Валидируем YAML файлы перед любой обработкой
        yaml_errors = self.validate_yaml_files(found_tasks)
        if yaml_errors:
            return {
                'added': [],
                'updated': [],
                'no_change': [],
                'duplicates': [],
                'error': 'YAML_VALIDATION_FAILED',
                'invalid_files': yaml_errors
            }

        # Загружаем текущий tasks_list.yaml
        tasks_list = self.load_tasks_list()

        # Проверяем дубликаты перед обработкой
        duplicates = self.validate_unique_task_ids(tasks_list)
        duplicates_removed = 0

        if duplicates:
            if auto_fix_duplicates:
                duplicates_removed = self.remove_duplicate_tasks(tasks_list)
            else:
                # Возвращаем информацию о дубликатах для ручного исправления
                return {
                    'added': [],
                    'updated': [],
                    'no_change': [],
                    'duplicates': duplicates,
                    'error': 'DUPLICATE_TASK_IDS_FOUND'
                }

        # Создаем индекс существующих задач
        existing_tasks = {}
        for section in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            for task in tasks_list.get(section, []):
                task_id = task.get('task_id')
                if task_id and task_id.startswith(req_id):
                    existing_tasks[task_id] = task

        changes = {
            'added': [],
            'updated': [],
            'no_change': []
        }

        # Обрабатываем каждую найденную задачу
        for task_data in found_tasks:
            if task_data.task_id in existing_tasks:
                # Задача существует - обновляем
                change_info = self._update_existing_task(tasks_list, existing_tasks[task_data.task_id], task_data)
                if change_info:
                    changes['updated'].append(change_info)
                else:
                    changes['no_change'].append(task_data.task_id)
            else:
                # Новая задача - добавляем
                self._add_new_task(tasks_list, task_data)
                changes['added'].append({
                    'task_id': task_data.task_id,
                    'status': task_data.status,
                    'reason': 'New task discovered from filesystem'
                })

        # Обновляем метаданные
        self._update_metadata(tasks_list)

        # Сохраняем изменения
        if changes['added'] or changes['updated'] or duplicates_removed > 0:
            self.save_tasks_list(tasks_list)
            # Авто-регенерация tasks_list.md
            self._regenerate_tasks_list_md()

        # Добавляем информацию о дубликатах в результат
        changes['duplicates'] = duplicates
        changes['duplicates_removed'] = duplicates_removed

        return changes

    def _regenerate_tasks_list_md(self):
        """Авто-регенерация tasks_list.md после обновления YAML"""
        try:
            # Import ReqStatusAnalyzer from req_status.py
            from req_status import ReqStatusAnalyzer
            analyzer = ReqStatusAnalyzer(self.project_root)
            analyzer.save_tasks_list_md()
        except ImportError:
            # Fallback: run as subprocess
            import subprocess
            script_path = Path(__file__).parent / 'req_status.py'
            subprocess.run(
                ['python3', str(script_path), 'generate-md'],
                cwd=self.project_root,
                capture_output=True
            )

    def _update_existing_task(self, tasks_list: Dict, existing_task: Dict, new_data: TaskData) -> Optional[Dict]:
        """
        Обновить существующую задачу.
        Возвращает информацию об изменениях или None если изменений нет.
        """
        changes = []
        old_status = existing_task.get('status', 'unknown')

        # Проверяем изменение статуса
        if old_status != new_data.status:
            changes.append(f"status: {old_status} → {new_data.status}")

        # Обновляем данные задачи
        self._update_task_dict(existing_task, new_data)

        # Перемещаем задачу в правильную секцию (даже если статус не изменился)
        old_section = self._find_task_section(tasks_list, new_data.task_id)
        new_section = self._status_to_section(new_data.status)

        if old_section != new_section:
            changes.append(f"section: {old_section} → {new_section}")

            # Удаляем из старой секции
            if old_section:
                tasks_list[old_section] = [
                    t for t in tasks_list.get(old_section, [])
                    if t.get('task_id') != new_data.task_id
                ]

            # Добавляем в новую секцию
            if new_section not in tasks_list:
                tasks_list[new_section] = []
            tasks_list[new_section].append(existing_task)

        # Проверяем изменения в файлах
        if self._has_file_changes(existing_task, new_data):
            changes.append("files updated")

        # Если были изменения, возвращаем информацию
        if changes:
            return {
                'task_id': new_data.task_id,
                'changes': changes,
                'old_status': old_status,
                'new_status': new_data.status
            }

        return None

    def _has_file_changes(self, existing_task: Dict, new_data: TaskData) -> bool:
        """Проверить, изменились ли файлы задачи"""
        # Проверяем основные файлы
        if existing_task.get('prp_file') != new_data.prp_file:
            return True
        if existing_task.get('result_report') != new_data.result_report:
            return True

        # Проверяем списки файлов
        old_reviews = set(existing_task.get('code_review_files', []))
        new_reviews = set(new_data.code_review_files)
        if old_reviews != new_reviews:
            return True

        old_bugs = set(existing_task.get('bug_report_files', []))
        new_bugs = set(new_data.bug_report_files)
        if old_bugs != new_bugs:
            return True

        return False

    def _find_task_section(self, tasks_list: Dict, task_id: str) -> Optional[str]:
        """Найти секцию, в которой находится задача"""
        for section in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            for task in tasks_list.get(section, []):
                if task.get('task_id') == task_id:
                    return section
        return None

    def _update_task_dict(self, task_dict: Dict, new_data: TaskData):
        """Обновить словарь задачи новыми данными"""
        # Обновляем статус
        task_dict['status'] = new_data.status

        # Обновляем файлы
        task_dict['task_file'] = new_data.task_file
        if new_data.prp_file:
            task_dict['prp_file'] = new_data.prp_file
        if new_data.result_report:
            task_dict['result_report'] = new_data.result_report

        # Обновляем списки файлов
        if new_data.code_review_files:
            task_dict['code_review_files'] = new_data.code_review_files
        if new_data.bug_report_files:
            task_dict['bug_report_files'] = new_data.bug_report_files

        # Обновляем метаданные, если они не были установлены
        if new_data.description and not task_dict.get('description'):
            task_dict['description'] = new_data.description
        if new_data.branch and not task_dict.get('branch'):
            task_dict['branch'] = new_data.branch
        if new_data.priority and task_dict.get('priority') == 'medium':
            task_dict['priority'] = new_data.priority
        if new_data.depends_on and not task_dict.get('depends_on'):
            task_dict['depends_on'] = new_data.depends_on

        # Обновляем jira_id если найден и ещё не установлен
        if new_data.jira_id and not task_dict.get('jira_id'):
            task_dict['jira_id'] = new_data.jira_id

        # Добавляем completed_at если статус completed
        if new_data.status == 'completed' and not task_dict.get('completed_at'):
            task_dict['completed_at'] = datetime.now().strftime('%Y-%m-%dT%H:%M:%SZ')

    def _add_new_task(self, tasks_list: Dict, task_data: TaskData):
        """Добавить новую задачу в tasks_list"""
        # Определяем секцию
        section = self._status_to_section(task_data.status)

        # Определяем branch: jira_id имеет приоритет
        if task_data.jira_id:
            default_branch = task_data.jira_id
        else:
            default_branch = f"feature/{task_data.task_id.lower()}"

        # Создаем словарь задачи
        task_dict = {
            'task_id': task_data.task_id,
            'branch': task_data.branch or default_branch,
            'description': task_data.description,
            'task_file': task_data.task_file,
            'status': task_data.status,
            'priority': task_data.priority,
            'depends_on': task_data.depends_on or []
        }

        # Добавляем jira_id если есть
        if task_data.jira_id:
            task_dict['jira_id'] = task_data.jira_id

        # Добавляем опциональные поля
        if task_data.prp_file:
            task_dict['prp_file'] = task_data.prp_file
        if task_data.result_report:
            task_dict['result_report'] = task_data.result_report
        if task_data.code_review_files:
            task_dict['code_review_files'] = task_data.code_review_files
        if task_data.bug_report_files:
            task_dict['bug_report_files'] = task_data.bug_report_files
        if task_data.status == 'completed':
            task_dict['completed_at'] = datetime.now().strftime('%Y-%m-%dT%H:%M:%SZ')

        # Добавляем в соответствующую секцию
        if section not in tasks_list:
            tasks_list[section] = []
        tasks_list[section].append(task_dict)

    def _status_to_section(self, status: str) -> str:
        """Преобразовать статус в название секции"""
        status_map = {
            'pending': 'pending_tasks',
            'prp_ready': 'pending_tasks',
            'in_progress': 'active_tasks',
            'active': 'active_tasks',
            'pending_review': 'active_tasks',
            'review_fixes': 'active_tasks',
            'completed': 'completed_tasks',
            'blocked': 'blocked_tasks'
        }
        return status_map.get(status, 'pending_tasks')

    def _update_metadata(self, tasks_list: Dict):
        """Обновить метаданные tasks_list"""
        tasks_list['metadata']['updated_at'] = datetime.now().strftime('%Y-%m-%dT%H:%M:%SZ')
        tasks_list['metadata']['total_pending'] = len(tasks_list.get('pending_tasks', []))
        tasks_list['metadata']['total_active'] = len(tasks_list.get('active_tasks', []))
        tasks_list['metadata']['total_completed'] = len(tasks_list.get('completed_tasks', []))
        tasks_list['metadata']['total_blocked'] = len(tasks_list.get('blocked_tasks', []))


def main():
    if len(sys.argv) < 2 or '--help' in sys.argv or '-h' in sys.argv:
        print("Usage: req_refresh.py <req_id> [--fix-duplicates] [--validate-only]")
        print("")
        print("Examples:")
        print("  req_refresh.py REQ-001")
        print("  req_refresh.py 1")
        print("  req_refresh.py task_management/tasks/REQ-001.md")
        print("  req_refresh.py REQ-001 --fix-duplicates    # Auto-fix duplicate task_ids")
        print("  req_refresh.py REQ-001 --validate-only     # Only check for duplicates")
        print("")
        print("What it does:")
        print("  - Scans filesystem for all task files")
        print("  - Finds related files (PRP, result, code review, bug reports)")
        print("  - Validates unique task_ids (detects duplicates)")
        print("  - Adds missing tasks to tasks_list.yaml")
        print("  - Updates task statuses based on file presence")
        print("  - Updates all file references")
        sys.exit(1)

    req_input = sys.argv[1]
    auto_fix_duplicates = '--fix-duplicates' in sys.argv
    validate_only = '--validate-only' in sys.argv

    # Найти корень проекта
    script_path = Path(__file__).resolve()
    project_root = script_path.parent.parent.parent

    service = ReqRefreshService(project_root)

    try:
        req_id = service.parse_req_id(req_input)

        # Режим только валидации
        if validate_only:
            print(f"🔍 Validating task_id uniqueness...")
            print("")
            duplicates = service.validate_unique_task_ids()
            if duplicates:
                print(f"❌ DUPLICATE_TASK_IDS_FOUND: {len(duplicates)} duplicate(s)")
                print("")
                _print_duplicates_report(duplicates)
                print("")
                print("💡 To auto-fix, run with --fix-duplicates flag")
                print("💡 Or manually edit .memory_bank/tasks_list.yaml")
                sys.exit(1)
            else:
                print("✅ All task_ids are unique. No duplicates found.")
                sys.exit(0)

        print(f"🔄 Refreshing {req_id}...")
        print(f"📁 Scanning: {service.tasks_dir}")
        print("")

        changes = service.refresh_request(req_id, auto_fix_duplicates=auto_fix_duplicates)

        # Проверяем на ошибки валидации YAML файлов
        if changes.get('error') == 'YAML_VALIDATION_FAILED':
            print("❌ YAML_VALIDATION_FAILED")
            print("")
            print("Следующие файлы содержат синтаксические ошибки и должны быть исправлены:")
            print("=" * 60)
            for err in changes.get('invalid_files', []):
                print(f"🔴 {err['file']}")
                print(f"   Task:  {err['task_id']}")
                print(f"   Error: {err['error']}")
                print("")
            print("=" * 60)
            print("")
            print("💡 Исправьте YAML файлы и повторно запустите:")
            print(f"   python3 .claude/scripts/req_refresh.py {req_id}")
            sys.exit(1)

        # Проверяем на дубликаты
        if changes.get('error') == 'DUPLICATE_TASK_IDS_FOUND':
            print("❌ DUPLICATE_TASK_IDS_FOUND")
            print("")
            print("Обнаружены дубликаты task_id в .memory_bank/tasks_list.yaml:")
            print("=" * 60)
            _print_duplicates_report(changes.get('duplicates', []))
            print("=" * 60)
            print("")
            print("💡 Чтобы исправить автоматически, запустите:")
            print(f"   python3 .claude/scripts/req_refresh.py {req_id} --fix-duplicates")
            print("")
            print("💡 Или исправьте вручную в файле:")
            print("   .memory_bank/tasks_list.yaml")
            sys.exit(1)

        if 'error' in changes and changes['error'] != 'DUPLICATE_TASK_IDS_FOUND':
            print(f"❌ {changes['error']}")
            sys.exit(1)

        # Вывод информации о исправленных дубликатах
        if changes.get('duplicates_removed', 0) > 0:
            print(f"🔧 Fixed {changes['duplicates_removed']} duplicate task_id(s)")
            print("")

        # Вывод результатов
        print(f"✅ Refresh completed for {req_id}")
        print("")

        if changes['added']:
            print(f"➕ Added {len(changes['added'])} new tasks:")
            for change in changes['added']:
                print(f"  - {change['task_id']} → {change['status']}")
                print(f"    Reason: {change['reason']}")
            print("")

        if changes['updated']:
            print(f"🔄 Updated {len(changes['updated'])} tasks:")
            for change in changes['updated']:
                print(f"  - {change['task_id']}: {change['old_status']} → {change['new_status']}")
                print(f"    Changes: {', '.join(change['changes'])}")
            print("")

        if not changes['added'] and not changes['updated'] and changes.get('duplicates_removed', 0) == 0:
            print("✨ No changes needed. All tasks are up to date.")
            print("")

        if changes['no_change']:
            print(f"✓ {len(changes['no_change'])} tasks unchanged")
            print("")

        # Статистика
        total_tasks = len(changes['added']) + len(changes['updated']) + len(changes['no_change'])
        print(f"📊 Total tasks processed: {total_tasks}")

    except Exception as e:
        print(f"❌ Error: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)


def _print_duplicates_report(duplicates: List[DuplicateTaskError]):
    """Вывести подробный отчёт о дубликатах для Claude"""
    for dup in duplicates:
        print(f"🔴 {dup.task_id}")
        print(f"   Найден в секциях: {', '.join(dup.sections)}")
        print(f"   Позиции: {', '.join(dup.line_hints)}")
        print("")
        print(f"   📝 Для исправления в .memory_bank/tasks_list.yaml:")
        print(f"      1. Найти все вхождения '{dup.task_id}' в файле")
        print(f"      2. Оставить только ОДНО вхождение (первое или наиболее актуальное)")
        print(f"      3. Удалить дублирующиеся блоки")
        print("")


if __name__ == '__main__':
    main()
