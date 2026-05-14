#!/usr/bin/env python3
"""
Task status tracking and lifecycle management for task_management system.

Features:
- Automatic status detection based on file presence
- Lifecycle tracking (execution, code_review, bug_fixing)
- File registration (PRP, result reports, code reviews, bug reports)
- Tasks list generation in YAML and Markdown formats
- Intelligent verdict determination from reports
"""

import os
import sys
import yaml
import argparse
from pathlib import Path
from datetime import datetime, timezone
from typing import Dict, List, Optional, Any
from collections import defaultdict

class TaskStatusAnalyzer:
    """Analyzes and tracks status and lifecycle of tasks."""

    def __init__(self, project_root: str = None):
        """
        Initialize analyzer.

        Args:
            project_root: Path to project root. If None, uses current directory.
        """
        self.project_root = Path(project_root or os.getcwd())
        self.tasks_list_path = self.project_root / '.memory_bank' / 'tasks_list.yaml'
        self.tasks_dir = self.project_root / 'task_management' / 'tasks'
        self.plans_dir = self.project_root / 'task_management' / 'plans'
        self.results_dir = self.project_root / 'task_management' / 'results'
        self.reviews_dir = self.project_root / 'task_management' / 'reviews'
        self.debug_reports_dir = self.project_root / 'task_management' / 'debug_reports'

        # Ensure tasks_list.yaml exists
        if not self.tasks_list_path.exists():
            self._create_empty_tasks_list()

    def _create_empty_tasks_list(self):
        """Create empty tasks_list.yaml with metadata."""
        self.tasks_list_path.parent.mkdir(parents=True, exist_ok=True)
        empty_data = {
            'metadata': {
                'template_version': '1.0',
                'updated_at': datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
                'total_pending': 0,
                'total_active': 0,
                'total_completed': 0,
                'total_blocked': 0
            },
            'pending_tasks': [],
            'active_tasks': [],
            'completed_tasks': [],
            'blocked_tasks': []
        }
        with open(self.tasks_list_path, 'w', encoding='utf-8') as f:
            yaml.dump(empty_data, f, allow_unicode=True, sort_keys=False, default_flow_style=False)

    def load_tasks(self) -> Dict:
        """Load tasks_list.yaml."""
        if not self.tasks_list_path.exists():
            self._create_empty_tasks_list()

        with open(self.tasks_list_path, 'r', encoding='utf-8') as f:
            return yaml.safe_load(f) or {}

    def save_tasks(self, data: Dict):
        """Save tasks_list.yaml."""
        # Update metadata
        if 'metadata' not in data:
            data['metadata'] = {}

        data['metadata']['updated_at'] = datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')
        data['metadata']['total_pending'] = len(data.get('pending_tasks', []))
        data['metadata']['total_active'] = len(data.get('active_tasks', []))
        data['metadata']['total_completed'] = len(data.get('completed_tasks', []))
        data['metadata']['total_blocked'] = len(data.get('blocked_tasks', []))

        with open(self.tasks_list_path, 'w', encoding='utf-8') as f:
            yaml.dump(data, f, allow_unicode=True, sort_keys=False, default_flow_style=False)

    def find_task(self, task_id: str) -> Optional[Dict]:
        """Find task by ID in tasks_list.yaml."""
        data = self.load_tasks()
        for category in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            for task in data.get(category, []):
                if task.get('task_id') == task_id:
                    return task
        return None

    def add_file_reference(self, task_id: str, file_type: str, file_path: str) -> Dict:
        """
        Add file reference to task.

        Args:
            task_id: Task identifier (e.g., REQ-001-TASK-001)
            file_type: Type of file (prp, result, code_review, bug_report, debug_report, task)
            file_path: Path to file (relative to project root)

        Returns:
            Dict with status and message
        """
        data = self.load_tasks()
        task = None
        task_category = None

        # Find task
        for category in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            for t in data.get(category, []):
                if t.get('task_id') == task_id:
                    task = t
                    task_category = category
                    break
            if task:
                break

        if not task:
            return {'error': 'Task not found', 'task_id': task_id}

        # Add file reference
        if file_type == 'prp':
            task['prp_file'] = file_path
        elif file_type == 'result':
            task['result_report'] = file_path
        elif file_type == 'code_review':
            if 'code_review_files' not in task:
                task['code_review_files'] = []
            if file_path not in task['code_review_files']:
                task['code_review_files'].append(file_path)
        elif file_type == 'bug_report':
            if 'bug_report_files' not in task:
                task['bug_report_files'] = []
            if file_path not in task['bug_report_files']:
                task['bug_report_files'].append(file_path)
        elif file_type == 'debug_report':
            if 'debug_report_files' not in task:
                task['debug_report_files'] = []
            if file_path not in task['debug_report_files']:
                task['debug_report_files'].append(file_path)
        elif file_type == 'task':
            task['task_file'] = file_path
        else:
            return {'error': f'Unknown file type: {file_type}'}

        self.save_tasks(data)
        return {
            'success': True,
            'task_id': task_id,
            'file_type': file_type,
            'file_path': file_path
        }

    def check_file_exists(self, file_path: str) -> bool:
        """Check if file exists relative to project root."""
        full_path = self.project_root / file_path
        return full_path.exists()

    def _get_target_category(self, status: str) -> str:
        """
        Determine target YAML category based on task status.

        Category mapping (aligned with CLAUDE.md status logic):
        - pending, prp_ready → pending_tasks (awaiting work)
        - in_progress, pending_review, review_fixes → active_tasks (work in progress)
        - completed → completed_tasks
        - blocked → blocked_tasks

        Args:
            status: Task status string

        Returns:
            Target category name (e.g., 'pending_tasks', 'completed_tasks')
        """
        if status in ['pending', 'prp_ready']:
            return 'pending_tasks'
        elif status in ['in_progress', 'pending_review', 'review_fixes']:
            return 'active_tasks'
        elif status == 'completed':
            return 'completed_tasks'
        elif status == 'blocked':
            return 'blocked_tasks'
        else:
            # Default to pending for unknown statuses
            return 'pending_tasks'

    def _move_task_between_categories(
        self,
        data: Dict,
        task: Dict,
        current_category: str,
        target_category: str
    ) -> bool:
        """
        Move task from current category to target category in YAML data.

        Args:
            data: Full tasks_list.yaml data dict
            task: Task dict to move
            current_category: Current category name (e.g., 'pending_tasks')
            target_category: Target category name (e.g., 'completed_tasks')

        Returns:
            True if task was moved, False if already in target category
        """
        if current_category == target_category:
            return False

        # Remove from current category
        if current_category in data and task in data[current_category]:
            data[current_category].remove(task)

        # Add to target category
        if target_category not in data:
            data[target_category] = []

        # Avoid duplicates
        task_id = task.get('task_id')
        existing_ids = [t.get('task_id') for t in data[target_category]]
        if task_id not in existing_ids:
            data[target_category].append(task)

        return True

    def detect_status(self, task: Dict) -> str:
        """
        Detect task status based on file presence and lifecycle.

        Status logic:
        - pending: No PRP file
        - prp_ready: PRP exists, not started
        - in_progress: Actively being worked on (result report exists but not completed)
        - pending_review: Execution completed, awaiting code review
        - review_fixes: Code review done, requires fixes
        - completed: Code review passed (APPROVED/ALL_FIXED)
        - blocked: Has blockers

        Returns:
            Status string
        """
        # Check blockers
        if task.get('blocked_by') or task.get('blocker'):
            return 'blocked'

        # Check PRP
        prp_file = task.get('prp_file')
        has_prp = prp_file and self.check_file_exists(prp_file)

        if not has_prp:
            return 'pending'

        # Check lifecycle
        lifecycle = task.get('lifecycle', {})
        execution = lifecycle.get('execution', {})
        code_review = lifecycle.get('code_review', {})

        execution_completed = execution.get('completed', False)
        code_review_completed = code_review.get('completed', False)
        verdict = code_review.get('verdict')

        # If code review is completed, execution was implicitly completed too
        if code_review_completed and not execution_completed:
            execution_completed = True

        # Execution not completed
        if not execution_completed:
            # Check if result report exists (in progress)
            result_file = task.get('result_report')
            if result_file and self.check_file_exists(result_file):
                return 'in_progress'
            # Check if code_review_files exist on disk (implies execution happened)
            cr_files = task.get('code_review_files', [])
            for cr_file in cr_files:
                if self.check_file_exists(cr_file):
                    execution_completed = True
                    break
            if not execution_completed:
                return 'prp_ready'

        # Execution completed, check code review
        if not code_review_completed:
            return 'pending_review'

        # Code review completed, check verdict
        if verdict in ['APPROVED', 'APPROVED_WITH_COMMENTS', 'ALL_FIXED']:
            return 'completed'
        elif verdict == 'CHANGES_REQUESTED':
            return 'review_fixes'
        else:
            # Unknown verdict, consider pending review
            return 'pending_review'

    def extract_report_metadata(self, report_path: Path) -> Dict:
        """
        Извлечь метаданные из code review, bug report или debug report.

        Returns:
            {
                'type': 'code_review' | 'bug_report' | 'debug_report',
                'verdict': str,
                'statistics': {
                    'critical': int,
                    'high': int,
                    'medium': int,
                    'low': int,
                    'warning': int,
                    'suggestion': int,
                    'total': int,
                    'fixed': int,
                    'skipped': int
                },
                'date': str,
                'error': str  # Присутствует только при ошибке парсинга
            }
        """
        try:
            with open(report_path, 'r', encoding='utf-8') as f:
                report = yaml.safe_load(f)
        except yaml.YAMLError as e:
            return {
                'type': 'unknown',
                'error': f"YAML parse error: {str(e)[:100]}",
                'file': str(report_path)
            }
        except Exception as e:
            return {
                'type': 'unknown',
                'error': f"Read error: {str(e)[:100]}",
                'file': str(report_path)
            }

        if report is None:
            return {
                'type': 'unknown',
                'error': "Empty YAML file",
                'file': str(report_path)
            }

        result = None

        # Code Review Report - Format 1: issues at root level (direct orchestrator reviews)
        if 'issues' in report:
            issues_raw = report['issues']
            if isinstance(issues_raw, list):
                # Flat list format: [{severity: ERROR/CRITICAL/WARNING/SUGGESTION, ...}, ...]
                sev = lambda s: s.get('severity', '').upper()
                n_critical = sum(1 for i in issues_raw if sev(i) == 'CRITICAL')
                n_error = sum(1 for i in issues_raw if sev(i) == 'ERROR')
                n_warning = sum(1 for i in issues_raw if sev(i) == 'WARNING')
                n_suggestion = sum(1 for i in issues_raw if sev(i) == 'SUGGESTION')
            else:
                # Dict format: {critical: [...], warnings: [...], suggestions: [...]}
                n_critical = len(issues_raw.get('critical', []))
                n_error = len(issues_raw.get('errors', []))
                n_warning = len(issues_raw.get('warnings', []))
                n_suggestion = len(issues_raw.get('suggestions', []))
            result = {
                'type': 'code_review',
                'verdict': report.get('verdict'),
                'statistics': {
                    'critical': n_critical + n_error,
                    'warning': n_warning,
                    'suggestion': n_suggestion
                },
                'date': report.get('metadata', {}).get('date') or report.get('date')
            }

        # Code Review Report - Format 2: code_review.issues (aggregated reports from report-aggregator)
        elif 'code_review' in report and 'issues' in report.get('code_review', {}):
            code_review = report['code_review']
            issues = code_review.get('issues', {})
            summary = code_review.get('summary', {})
            # verdict can be at root level or inside code_review
            verdict = report.get('verdict') or code_review.get('verdict')
            result = {
                'type': 'code_review',
                'verdict': verdict,
                'statistics': {
                    'critical': summary.get('total_critical', len(issues.get('critical', []))),
                    'warning': summary.get('total_warnings', len(issues.get('warnings', []))),
                    'suggestion': summary.get('total_suggestions', len(issues.get('suggestions', [])))
                },
                'date': report.get('metadata', {}).get('date')
            }

        # Code Review Report - Format 3: all_issues at root (another aggregated format)
        elif 'all_issues' in report:
            all_issues = report.get('all_issues', {})
            summary = report.get('summary', {})
            result = {
                'type': 'code_review',
                'verdict': report.get('verdict'),
                'statistics': {
                    'critical': summary.get('total_critical_issues', len(all_issues.get('critical', []))),
                    'warning': summary.get('total_warning_issues', len(all_issues.get('warnings', []))),
                    'suggestion': summary.get('total_suggestion_issues', len(all_issues.get('suggestions', [])))
                },
                'date': report.get('metadata', {}).get('date')
            }

        # Bug Report
        elif 'bugs' in report:
            bug_summary = report.get('summary', {}).get('by_priority', {})
            result = {
                'type': 'bug_report',
                'verdict': report.get('verdict'),
                'statistics': {
                    'critical': bug_summary.get('critical', 0),
                    'high': bug_summary.get('high', 0),
                    'medium': bug_summary.get('medium', 0),
                    'low': bug_summary.get('low', 0),
                    'total': sum(bug_summary.values()) if bug_summary else 0
                },
                'date': report.get('metadata', {}).get('date')
            }

        # Debug Report
        elif 'debug_results' in report or 'fixes' in report:
            statistics = report.get('summary', {}).get('statistics', {})
            result = {
                'type': 'debug_report',
                'verdict': report.get('verdict'),
                'statistics': {
                    'total': statistics.get('total_issues', 0),
                    'fixed': statistics.get('fixed', 0),
                    'skipped': statistics.get('skipped', 0)
                },
                'date': report.get('metadata', {}).get('date')
            }

        # Code Review Report - Format 4: verdict-only (DevOps/simple reviews without issues list)
        # Fallback for CODE-REVIEW files that have verdict but no issues key
        elif 'verdict' in report and report.get('metadata', {}).get('task_id'):
            summary = report.get('summary', {})
            result = {
                'type': 'code_review',
                'verdict': report.get('verdict'),
                'statistics': {
                    'critical': summary.get('critical_count', 0),
                    'warning': summary.get('warning_count', 0),
                    'suggestion': summary.get('suggestion_count', 0)
                },
                'date': report.get('metadata', {}).get('date') or report.get('metadata', {}).get('review_date')
            }

        # Code Review Report - Format 5: top-level critical/warnings/suggestions
        # (code-reviewer agent format with task_id at root, not inside metadata)
        elif 'verdict' in report and ('critical' in report or 'warnings' in report or 'suggestions' in report):
            crit = report.get('critical', [])
            warn = report.get('warnings', [])
            sugg = report.get('suggestions', [])
            result = {
                'type': 'code_review',
                'verdict': report.get('verdict'),
                'statistics': {
                    'critical': len(crit) if isinstance(crit, list) else 0,
                    'warning': len(warn) if isinstance(warn, list) else 0,
                    'suggestion': len(sugg) if isinstance(sugg, list) else 0
                },
                'date': report.get('date')
            }

        if result is None:
            return {
                'type': 'unknown',
                'error': f"Unknown report format. Expected keys: issues, code_review.issues, all_issues, bugs, debug_results, fixes, or (verdict + metadata.task_id)",
                'file': str(report_path)
            }

        # Post-processing: apply review_fixes overlay for code reviews.
        # review_fixes is a section in CODE-REVIEW.yaml that tracks applied fixes
        # (e.g. review_fixes.status: "ALL_FIXED" with list of fixed issues).
        # Without this, scripts only know about fixes via DEBUG-REPORT files.
        review_fixes = report.get('review_fixes')
        if review_fixes and result.get('type') == 'code_review':
            rf_status = str(review_fixes.get('status', '')).upper()
            if rf_status == 'ALL_FIXED':
                result['original_verdict'] = result.get('verdict')
                result['verdict'] = 'ALL_FIXED'
                stats = result.get('statistics', {})
                stats['critical_fixed'] = stats.get('critical', 0)
                stats['warning_fixed'] = stats.get('warning', 0)
                stats['suggestion_fixed'] = stats.get('suggestion', 0)

        return result

    def refresh_lifecycle(self, task_id: str) -> Dict:
        """
        Refresh lifecycle tracking for a task based on existing reports.

        Lifecycle tracking:
        - execution: completed status, completion date
        - code_review: verdict, issue counts, completion date
        - bug_fixing: has_bugs, bugs_fixed, bug severity statistics

        Returns:
            Dict with updated lifecycle
        """
        data = self.load_tasks()
        task = None
        task_category = None

        # Find task
        for category in ['pending_tasks', 'active_tasks', 'completed_tasks', 'blocked_tasks']:
            for t in data.get(category, []):
                if t.get('task_id') == task_id:
                    task = t
                    task_category = category
                    break
            if task:
                break

        if not task:
            return {'error': 'Task not found', 'task_id': task_id}

        # Инициализировать lifecycle если нет
        if 'lifecycle' not in task:
            task['lifecycle'] = {
                'execution': {},
                'code_review': {},
                'bug_fixing': {}
            }

        # Ensure all sub-keys exist
        if 'execution' not in task['lifecycle']:
            task['lifecycle']['execution'] = {}
        if 'code_review' not in task['lifecycle']:
            task['lifecycle']['code_review'] = {}
        if 'bug_fixing' not in task['lifecycle']:
            task['lifecycle']['bug_fixing'] = {}

        # 1. Execution
        if task.get('result_report') and self.check_file_exists(task['result_report']):
            task['lifecycle']['execution']['completed'] = True
            # Извлечь дату из completed_at если есть, иначе из файла
            if task.get('completed_at'):
                task['lifecycle']['execution']['completed_at'] = task['completed_at']
            else:
                # Используем дату модификации файла
                result_path = self.project_root / task['result_report']
                task['lifecycle']['execution']['completed_at'] = datetime.fromtimestamp(
                    result_path.stat().st_mtime
                ).strftime('%Y-%m-%dT%H:%M:%SZ')
        else:
            task['lifecycle']['execution']['completed'] = False

        # 2. Code Review
        code_review_files = task.get('code_review_files', [])
        parse_warnings = []  # Собираем предупреждения о невалидных файлах

        if code_review_files:
            # Берем последний отчет (самый свежий)
            latest_review = code_review_files[-1]
            if self.check_file_exists(latest_review):
                metadata = self.extract_report_metadata(self.project_root / latest_review)

                # Проверяем на ошибку парсинга
                if metadata.get('error'):
                    task['lifecycle']['code_review']['completed'] = False
                    task['lifecycle']['code_review']['error'] = metadata['error']
                    parse_warnings.append(f"Code review YAML error in {latest_review}: {metadata['error']}")
                else:
                    stats = metadata.get('statistics', {})
                    task['lifecycle']['code_review']['completed'] = True
                    task['lifecycle']['code_review']['completed_at'] = metadata.get('date')
                    task['lifecycle']['code_review']['verdict'] = metadata.get('verdict')
                    # original_verdict: pre-review_fixes verdict (if review_fixes applied,
                    # extract_report_metadata sets 'original_verdict'; otherwise same as verdict)
                    task['lifecycle']['code_review']['original_verdict'] = metadata.get('original_verdict') or metadata.get('verdict')
                    task['lifecycle']['code_review']['critical_issues'] = stats.get('critical', 0)
                    task['lifecycle']['code_review']['warning_issues'] = stats.get('warning', 0)
                    task['lifecycle']['code_review']['suggestion_issues'] = stats.get('suggestion', 0)
                    # Store fixed counts from review_fixes (if present)
                    if 'critical_fixed' in stats:
                        task['lifecycle']['code_review']['critical_issues_fixed'] = stats['critical_fixed']
                    if 'warning_fixed' in stats:
                        task['lifecycle']['code_review']['warning_issues_fixed'] = stats['warning_fixed']
                    if 'suggestion_fixed' in stats:
                        task['lifecycle']['code_review']['suggestion_issues_fixed'] = stats['suggestion_fixed']
                    # Remove error field if present (cleanup from previous failed attempts)
                    task['lifecycle']['code_review'].pop('error', None)
            else:
                task['lifecycle']['code_review']['completed'] = False
        else:
            task['lifecycle']['code_review']['completed'] = False

        # 2.5. Check DEBUG-REPORT to update verdict if bugs are fixed
        debug_report_files = task.get('debug_report_files', [])
        if debug_report_files and task['lifecycle']['code_review'].get('completed'):
            # Берем последний debug report (самый свежий)
            latest_debug = debug_report_files[-1]
            if self.check_file_exists(latest_debug):
                debug_metadata = self.extract_report_metadata(self.project_root / latest_debug)
                if debug_metadata.get('error'):
                    parse_warnings.append(f"Debug report YAML error in {latest_debug}: {debug_metadata['error']}")
                elif debug_metadata.get('type') == 'debug_report':
                    debug_verdict = debug_metadata.get('verdict')
                    # Если все баги исправлены, обновляем verdict в code_review
                    # RESOLVED и ALL_FIXED оба означают "все применимые замечания устранены"
                    if debug_verdict in ('ALL_FIXED', 'RESOLVED'):
                        task['lifecycle']['code_review']['verdict'] = 'ALL_FIXED'
                        task['lifecycle']['code_review']['debug_completed_at'] = debug_metadata.get('date')

        # 3. Bug Fixing
        bug_report_files = task.get('bug_report_files', [])
        if bug_report_files:
            latest_bug_report = bug_report_files[-1]
            if self.check_file_exists(latest_bug_report):
                metadata = self.extract_report_metadata(self.project_root / latest_bug_report)

                if metadata.get('error'):
                    task['lifecycle']['bug_fixing']['has_bugs'] = False
                    task['lifecycle']['bug_fixing']['error'] = metadata['error']
                    parse_warnings.append(f"Bug report YAML error in {latest_bug_report}: {metadata['error']}")
                else:
                    task['lifecycle']['bug_fixing']['has_bugs'] = True
                    task['lifecycle']['bug_fixing']['critical_bugs'] = metadata.get('statistics', {}).get('critical', 0)
                    task['lifecycle']['bug_fixing']['high_bugs'] = metadata.get('statistics', {}).get('high', 0)
                    task['lifecycle']['bug_fixing']['medium_bugs'] = metadata.get('statistics', {}).get('medium', 0)
                    task['lifecycle']['bug_fixing']['low_bugs'] = metadata.get('statistics', {}).get('low', 0)

                    # Check if bugs are fixed via debug report
                    if debug_report_files:
                        latest_debug = debug_report_files[-1]
                        if self.check_file_exists(latest_debug):
                            debug_metadata = self.extract_report_metadata(self.project_root / latest_debug)
                            if not debug_metadata.get('error') and debug_metadata.get('type') == 'debug_report':
                                verdict = debug_metadata.get('verdict')
                                task['lifecycle']['bug_fixing']['bugs_fixed'] = verdict in ('ALL_FIXED', 'RESOLVED')
                                task['lifecycle']['bug_fixing']['debug_verdict'] = verdict
                    else:
                        task['lifecycle']['bug_fixing']['bugs_fixed'] = False

                    # Remove error field if present
                    task['lifecycle']['bug_fixing'].pop('error', None)
        else:
            task['lifecycle']['bug_fixing']['has_bugs'] = False
            # If no bug reports but debug report exists with ALL_FIXED/RESOLVED, mark as fixed
            if debug_report_files:
                latest_debug = debug_report_files[-1]
                if self.check_file_exists(latest_debug):
                    debug_metadata = self.extract_report_metadata(self.project_root / latest_debug)
                    if not debug_metadata.get('error') and debug_metadata.get('type') == 'debug_report':
                        verdict = debug_metadata.get('verdict')
                        task['lifecycle']['bug_fixing']['bugs_fixed'] = verdict in ('ALL_FIXED', 'RESOLVED')
                        task['lifecycle']['bug_fixing']['debug_verdict'] = verdict
                    else:
                        task['lifecycle']['bug_fixing']['bugs_fixed'] = False
                else:
                    task['lifecycle']['bug_fixing']['bugs_fixed'] = False
            else:
                task['lifecycle']['bug_fixing']['bugs_fixed'] = False

        # Update status based on lifecycle
        old_status = task.get('status')
        new_status = self.detect_status(task)
        task['status'] = new_status

        # Move task to appropriate category based on new status
        old_category = task_category
        new_category = self._get_target_category(new_status)
        moved = self._move_task_between_categories(data, task, old_category, new_category)

        self.save_tasks(data)

        result = {
            'success': True,
            'task_id': task_id,
            'old_status': old_status,
            'new_status': new_status,
            'old_category': old_category,
            'new_category': new_category,
            'category_changed': moved,
            'lifecycle': task['lifecycle']
        }

        if parse_warnings:
            result['warnings'] = parse_warnings

        return result

    def generate_markdown(self) -> str:
        """
        Generate tasks_list.md from tasks_list.yaml in table format.

        Tables:
        1. New Tasks (no PRP) - tasks without PRP file
        2. Pending Tasks - tasks with PRP but not completed
        3. Active Tasks - tasks in progress
        4. Completed Tasks - fully completed tasks
        5. Blocked Tasks - blocked tasks

        Columns:
        1. Request ID
        2. Task ID
        3. Description
        4. Status
        5. PRP (checkbox)
        6. Execution (checkbox)
        7. Review (checkbox + critical/other counts)
        8. Debug (checkbox)
        9. Files (clickable links)

        Returns:
            Path to generated markdown file
        """
        from datetime import timezone
        data = self.load_tasks()
        md_path = self.project_root / '.memory_bank' / 'tasks_list.md'

        # Status icons
        status_icons = {
            'pending': '⏳',
            'task_ready': '⏳',
            'prp_ready': '📋',
            'in_progress': '🔄',
            'pending_review': '👀',
            'review_fixes': '🔧',
            'completed': '✅',
            'blocked': '🚫'
        }

        def get_request_id(task_id):
            """Extract request ID from task ID (e.g., REQ-001-TASK-005 -> REQ-001)."""
            parts = task_id.split('-TASK-')
            return parts[0] if len(parts) > 1 else task_id

        def get_jira_id(task):
            """Get Jira ID if present."""
            return task.get('jira_id', '')

        def has_prp(task):
            """Check if task has PRP file."""
            prp_file = task.get('prp_file')
            return prp_file and self.check_file_exists(prp_file)

        def get_prp_status(task):
            """Check if PRP file exists."""
            return '✅' if has_prp(task) else '-'

        def get_execution_status(task):
            """Check if execution is completed (result report exists)."""
            lifecycle = task.get('lifecycle', {})
            execution = lifecycle.get('execution', {})
            if execution.get('completed'):
                return '✅'
            result_report = task.get('result_report')
            if result_report and self.check_file_exists(result_report):
                return '✅'
            return '-'

        def get_review_status(task):
            """Get code review status - just shows if review is done."""
            lifecycle = task.get('lifecycle', {})
            code_review = lifecycle.get('code_review', {})
            if code_review.get('completed'):
                return '✅'
            # Fallback: check if code_review_files exist on disk
            cr_files = task.get('code_review_files', [])
            for cr_file in cr_files:
                if self.check_file_exists(cr_file):
                    return '✅'
            return '-'

        def get_issues_status(task):
            """Get issues status with icons for critical/warning/suggestion.

            Icons:
            - 🔴 critical (must fix)
            - ⚠️ warning (should fix)
            - 💡 suggestion (optional)

            Format: 🔴 0/3 ⚠️ 0/5 💡 0/2  (fixed/found)
            Format: 🔴 3 ⚠️ 5 💡 2        (unfixed issues, no denominator)
            """
            lifecycle = task.get('lifecycle', {})
            code_review = lifecycle.get('code_review', {})
            if not code_review.get('completed'):
                return '-'

            verdict = code_review.get('verdict', 'UNKNOWN')
            critical = code_review.get('critical_issues', 0)
            warning = code_review.get('warning_issues', 0)
            suggestion = code_review.get('suggestion_issues', 0)

            # No issues at all
            if critical == 0 and warning == 0 and suggestion == 0:
                return '-'

            parts = []

            # Determine if issues were fixed.
            # APPROVED/APPROVED_WITH_COMMENTS means reviewer confirmed all issues
            # were addressed — treat as all fixed.
            bug_fixing = lifecycle.get('bug_fixing', {})
            all_fixed = (
                verdict in ('ALL_FIXED', 'APPROVED', 'APPROVED_WITH_COMMENTS')
                or bug_fixing.get('bugs_fixed')
                or code_review.get('critical_issues_fixed', 0) >= critical > 0
            )

            if all_fixed:
                # Show fixed/found format
                critical_fixed = code_review.get('critical_issues_fixed', critical)
                warning_fixed = code_review.get('warning_issues_fixed', warning)
                suggestion_fixed = code_review.get('suggestion_issues_fixed', suggestion)
                if critical > 0:
                    parts.append(f"🔴 {critical_fixed}/{critical}")
                if warning > 0:
                    parts.append(f"⚠️ {warning_fixed}/{warning}")
                if suggestion > 0:
                    parts.append(f"💡 {suggestion_fixed}/{suggestion}")
            else:
                # Show current unfixed issues
                if critical > 0:
                    parts.append(f"🔴 {critical}")
                if warning > 0:
                    parts.append(f"⚠️ {warning}")
                if suggestion > 0:
                    parts.append(f"💡 {suggestion}")

            return " ".join(parts) if parts else '-'

        def are_issues_fixed(task):
            """
            Check if all critical and warning issues are fixed.
            Rules for completed:
            - ALL_FIXED verdict: all issues fixed
            - APPROVED / APPROVED_WITH_COMMENTS: reviewer confirmed issues addressed
            - CHANGES_REQUESTED with critical_issues_fixed >= critical_issues: fixed
            - CHANGES_REQUESTED with unfixed critical: NOT fixed
            - bug_fixing.bugs_fixed=true also indicates all issues resolved
            - review_fixes.status=ALL_FIXED in review file: all issues resolved
            """
            lifecycle = task.get('lifecycle', {})
            code_review = lifecycle.get('code_review', {})

            # If lifecycle has no review data, try to read from review files directly
            if not code_review.get('completed'):
                cr_files = task.get('code_review_files', [])
                for cr_file in cr_files:
                    full_path = self.project_root / cr_file
                    if full_path.exists():
                        try:
                            with open(full_path, 'r', encoding='utf-8') as f_cr:
                                report = yaml.safe_load(f_cr)
                            if report:
                                meta = self.extract_report_metadata(full_path)
                                v = meta.get('verdict', '')
                                if v in ('ALL_FIXED', 'APPROVED', 'APPROVED_WITH_COMMENTS'):
                                    return True
                        except Exception:
                            pass
                return True  # No review = no issues to fix

            verdict = code_review.get('verdict', 'UNKNOWN')
            critical = code_review.get('critical_issues', 0)
            critical_fixed = code_review.get('critical_issues_fixed', 0)

            # ALL_FIXED - all issues were fixed
            if verdict == 'ALL_FIXED':
                return True

            # Bug fixing lifecycle completed — all issues resolved
            bug_fixing = lifecycle.get('bug_fixing', {})
            if bug_fixing.get('bugs_fixed'):
                return True

            # APPROVED / APPROVED_WITH_COMMENTS - reviewer confirmed all issues addressed
            if verdict in ('APPROVED', 'APPROVED_WITH_COMMENTS'):
                return True

            # CHANGES_REQUESTED - check if critical issues were fixed
            if verdict == 'CHANGES_REQUESTED':
                if critical > 0 and critical_fixed >= critical:
                    return True
                return critical == 0

            # Unknown verdict - assume not fixed if has critical issues
            return critical == 0

        def get_debug_status(task):
            """Check if debug report exists and bugs are fixed."""
            lifecycle = task.get('lifecycle', {})
            bug_fixing = lifecycle.get('bug_fixing', {})
            if bug_fixing.get('bugs_fixed'):
                return '✅'
            debug_files = task.get('debug_report_files', [])
            if debug_files:
                return '🔧'
            return '-'

        def truncate(text, max_len=40):
            """Truncate text with ellipsis."""
            if len(text) > max_len:
                return text[:max_len-3] + "..."
            return text

        def get_files_list(task):
            """Get clickable file links with icons and lowercase text (markdown format).

            Format: 📄 [task](path) 📋 [prp](path)
            Links use ../ prefix because tasks_list.md is in .memory_bank/ directory.
            """
            files = []

            def make_link(icon, name, path):
                """Create icon + markdown link with ../ prefix for paths outside .memory_bank/."""
                if path.startswith('.memory_bank/'):
                    # Same directory, use relative path without ../
                    rel_path = path[len('.memory_bank/'):]
                    return f"{icon} [{name}]({rel_path})"
                else:
                    # Other directories, need ../
                    return f"{icon} [{name}](../{path})"

            if task.get('task_file') and self.check_file_exists(task['task_file']):
                files.append(make_link('📄', 'task', task['task_file']))
            if task.get('prp_file') and self.check_file_exists(task['prp_file']):
                files.append(make_link('📋', 'prp', task['prp_file']))
            if task.get('result_report') and self.check_file_exists(task['result_report']):
                files.append(make_link('✅', 'result', task['result_report']))
            # Only show last code review file
            cr_files = task.get('code_review_files', [])
            if cr_files:
                cr_file = cr_files[-1]  # last one
                if self.check_file_exists(cr_file):
                    files.append(make_link('🔍', 'review', cr_file))
            # Only show last debug report file
            dr_files = task.get('debug_report_files', [])
            if dr_files:
                dr_file = dr_files[-1]  # last one
                if self.check_file_exists(dr_file):
                    files.append(make_link('🐛', 'debug', dr_file))
            return " ".join(files) if files else "-"

        def is_truly_completed(task):
            """
            Check if task is truly completed:
            1) Has PRP
            2) Execution completed
            3) Review completed
            4) All critical and warning issues fixed
            """
            if not has_prp(task):
                return False
            if get_execution_status(task) != '✅':
                return False
            if get_review_status(task) != '✅':
                return False
            if not are_issues_fixed(task):
                return False
            return True

        def sort_by_task_id(tasks):
            """Sort tasks by task ID."""
            def task_sort_key(task):
                task_id = task.get('task_id', 'UNKNOWN')
                # Extract numeric parts for proper sorting
                # REQ-001-TASK-005 -> (1, 5)
                parts = task_id.replace('REQ-', '').replace('TASK-', '').split('-')
                try:
                    return tuple(int(p) for p in parts if p.isdigit())
                except ValueError:
                    return (999, 999)
            return sorted(tasks, key=task_sort_key)

        def write_table(f, tasks, title):
            """Write a task table."""
            if not tasks:
                return

            f.write(f"## {title}\n\n")

            # Table header with Jira and Issues columns
            f.write("| Request | Task ID | Jira | Description | Status | PRP | Execution | Review | Issues | Debug | Files |\n")
            f.write("|---------|---------|------|-------------|--------|-----|-----------|--------|--------|-------|-------|\n")

            # Sort tasks by Task ID
            sorted_tasks = sort_by_task_id(tasks)

            for task in sorted_tasks:
                task_id = task.get('task_id', 'UNKNOWN')
                request_id = get_request_id(task_id)
                status = task.get('status', 'unknown')
                description = task.get('description', 'No description')
                jira_id = get_jira_id(task)

                status_icon = status_icons.get(status, '❓')

                # Create links for Request and Task ID
                req_file = f"task_management/tasks/{request_id}.md"
                task_file = task.get('task_file', f"task_management/tasks/{task_id}.md")
                request_link = f"[{request_id}](../{req_file})" if self.check_file_exists(req_file) else request_id
                task_link = f"[{task_id}](../{task_file})" if self.check_file_exists(task_file) else task_id

                prp_status = get_prp_status(task)
                exec_status = get_execution_status(task)
                review_status = get_review_status(task)
                issues_status = get_issues_status(task)
                debug_status = get_debug_status(task)
                files_list = get_files_list(task)

                # Write row (no truncation for description, status icon only)
                f.write(f"| {request_link} | {task_link} | {jira_id} | {description} | {status_icon} | {prp_status} | {exec_status} | {review_status} | {issues_status} | {debug_status} | {files_list} |\n")

            f.write("\n")

        with open(md_path, 'w', encoding='utf-8') as f:
            f.write("# Tasks List\n\n")
            f.write(f"**Generated:** {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')}\n\n")

            metadata = data.get('metadata', {})
            f.write("## Summary\n\n")
            f.write(f"| Metric | Count |\n")
            f.write(f"|--------|-------|\n")
            f.write(f"| ⏳ Pending | {metadata.get('total_pending', 0)} |\n")
            f.write(f"| 🔄 Active | {metadata.get('total_active', 0)} |\n")
            f.write(f"| ✅ Completed | {metadata.get('total_completed', 0)} |\n")
            f.write(f"| 🚫 Blocked | {metadata.get('total_blocked', 0)} |\n\n")

            # Legend section in table format
            f.write("## Legend\n\n")

            f.write("| Status | | Issues | | Files | |\n")
            f.write("|--------|---|--------|---|-------|---|\n")
            f.write("| ⏳ | pending | 🔴 | critical (must fix) | 📄 | task |\n")
            f.write("| 📋 | prp_ready | ⚠️ | warning (should fix) | 📋 | prp |\n")
            f.write("| 🔄 | in_progress | 💡 | suggestion (optional) | ✅ | result |\n")
            f.write("| ✅ | completed | | | 🔍 | review |\n")
            f.write("| 🚫 | blocked | | | 🐛 | debug |\n")
            f.write("| 👀 | pending_review | | | | |\n")
            f.write("| 🔧 | review_fixes | | | | |\n\n")

            f.write("**Issues format:** `X` - найдено, `X/X` - исправлено/найдено\n\n")

            # Collect all tasks
            all_pending = data.get('pending_tasks', [])
            all_active = data.get('active_tasks', [])
            all_completed = data.get('completed_tasks', [])
            all_blocked = data.get('blocked_tasks', [])

            # Split pending into "New" (no PRP) and "Pending" (has PRP)
            new_tasks = [t for t in all_pending if not has_prp(t)]
            pending_with_prp = [t for t in all_pending if has_prp(t)]

            # Split active into tasks with PRP (truly active) and without PRP (new tasks)
            active_with_prp = [t for t in all_active if has_prp(t)]
            active_without_prp = [t for t in all_active if not has_prp(t)]
            new_tasks.extend(active_without_prp)

            # Check completed tasks - move incorrectly completed to pending
            truly_completed = []
            not_truly_completed = []
            for task in all_completed:
                if is_truly_completed(task):
                    truly_completed.append(task)
                else:
                    not_truly_completed.append(task)

            # Add not truly completed to pending_with_prp
            pending_with_prp.extend(not_truly_completed)

            # Write tables
            write_table(f, new_tasks, "🆕 New Tasks (no PRP)")
            write_table(f, pending_with_prp, "⏳ Pending Tasks")
            write_table(f, active_with_prp, "🔄 Active Tasks")
            write_table(f, truly_completed, "✅ Completed Tasks")
            write_table(f, all_blocked, "🚫 Blocked Tasks")

        return str(md_path)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(description='Task status tracking and lifecycle management')
    parser.add_argument('command', choices=[
        'add-file',
        'refresh-lifecycle',
        'generate-md',
        'show-task'
    ], help='Command to execute')
    parser.add_argument('task_id', nargs='?', help='Task ID (e.g., REQ-001-TASK-001)')
    parser.add_argument('file_type', nargs='?', help='File type (prp, result, code_review, bug_report, debug_report, task)')
    parser.add_argument('file_path', nargs='?', help='Path to file (relative to project root)')
    parser.add_argument('--project-root', help='Project root directory (default: current directory)')

    args = parser.parse_args()

    analyzer = TaskStatusAnalyzer(project_root=args.project_root)

    try:
        if args.command == 'add-file':
            if not args.task_id or not args.file_type or not args.file_path:
                print("❌ Error: add-file requires task_id, file_type, and file_path")
                sys.exit(1)

            result = analyzer.add_file_reference(args.task_id, args.file_type, args.file_path)
            if result.get('error'):
                print(f"❌ Error: {result['error']}")
                sys.exit(1)
            else:
                print(f"✅ Added {args.file_type} file reference to {args.task_id}")
                print(f"   File: {args.file_path}")

        elif args.command == 'refresh-lifecycle':
            if not args.task_id:
                print("❌ Error: refresh-lifecycle requires task_id")
                sys.exit(1)

            result = analyzer.refresh_lifecycle(args.task_id)
            if result.get('error'):
                print(f"❌ Error: {result['error']}")
                sys.exit(1)
            else:
                print(f"✅ Refreshed lifecycle for {args.task_id}")
                print(f"   Status: {result['old_status']} → {result['new_status']}")
                if result.get('category_changed'):
                    print(f"   Category: {result['old_category']} → {result['new_category']}")

                lifecycle = result.get('lifecycle', {})
                execution = lifecycle.get('execution', {})
                code_review = lifecycle.get('code_review', {})
                bug_fixing = lifecycle.get('bug_fixing', {})

                if execution.get('completed'):
                    print(f"   Execution: ✅ completed")
                else:
                    print(f"   Execution: ⏳ not completed")

                if code_review.get('completed'):
                    verdict = code_review.get('verdict', 'UNKNOWN')
                    critical = code_review.get('critical_issues', 0)
                    print(f"   Code Review: {verdict} (Critical: {critical})")
                else:
                    print(f"   Code Review: ⏳ not completed")

                if bug_fixing.get('has_bugs'):
                    fixed = bug_fixing.get('bugs_fixed', False)
                    critical = bug_fixing.get('critical_bugs', 0)
                    status = "✅ fixed" if fixed else "🔧 in progress"
                    print(f"   Bug Fixing: {status} (Critical: {critical})")

                if result.get('warnings'):
                    print("\n⚠️  Warnings:")
                    for warning in result['warnings']:
                        print(f"   - {warning}")

                # Auto-regenerate tasks_list.md after lifecycle update
                md_path = analyzer.generate_markdown()
                print(f"   📄 Regenerated {md_path}")

        elif args.command == 'generate-md':
            md_path = analyzer.generate_markdown()
            print(f"✅ Generated {md_path}")

        elif args.command == 'show-task':
            if not args.task_id:
                print("❌ Error: show-task requires task_id")
                sys.exit(1)

            task = analyzer.find_task(args.task_id)
            if not task:
                print(f"❌ Error: Task {args.task_id} not found")
                sys.exit(1)

            print(yaml.dump({args.task_id: task}, allow_unicode=True, sort_keys=False, default_flow_style=False))

    except Exception as e:
        print(f"❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
