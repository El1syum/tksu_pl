#!/usr/bin/env python3
"""End-to-end HTTP check against a running app. Uses only Python's standard library.

Creates two disposable accounts, removes their expenses, and logs out.
The resulting empty accounts remain; their exact emails are printed for cleanup.
"""
import csv
from datetime import date
import io
import json
import re
import secrets
import sys
from html import unescape
from http.cookiejar import CookieJar
from urllib.error import HTTPError
from urllib.parse import urlencode
from urllib.request import HTTPCookieProcessor, Request, build_opener

base = (sys.argv[1] if len(sys.argv) > 1 else 'http://localhost:8080').rstrip('/')


class Client:
    def __init__(self):
        self.opener = build_opener(HTTPCookieProcessor(CookieJar()))
        self.csrf = ''

    def get(self, path):
        with self.opener.open(base + path, timeout=20) as response:
            body = unescape(response.read().decode('utf-8-sig'))
            match = re.search(r'name="csrf" value="([^"]+)"', body)
            if match:
                self.csrf = match.group(1)
            return body

    def post(self, path, values):
        data = urlencode(dict(values, csrf=self.csrf)).encode()
        request = Request(base + path, data=data, headers={'Origin': base})
        with self.opener.open(request, timeout=20) as response:
            body = unescape(response.read().decode())
            match = re.search(r'name="csrf" value="([^"]+)"', body)
            if match:
                self.csrf = match.group(1)
            return body

    def register(self, email, password):
        self.get('/register')
        self.post('/register', {'email': email, 'password': password, 'password_confirm': password})
        self.post('/login', {'email': email, 'password': password})


def check(condition, message):
    if not condition:
        raise AssertionError(message)


a, b = Client(), Client()
tag = secrets.token_hex(5)
emails = [f'smoke-{tag}-a@example.com', f'smoke-{tag}-b@example.com']
print(json.dumps({'test_accounts': emails}), file=sys.stderr, flush=True)
password = secrets.token_urlsafe(24)
check(a.get('/ping') == 'pong', 'ping')
for client, email in zip([a, b], emails):
    client.register(email, password)
    check('У каждой истории есть начало' in client.get('/expenses'), 'empty state')
body = a.post('/expenses', {'amount': '1250,75', 'category_id': '1', 'description': 'Smoke test expense', 'date': '2026-09-15'})
expense_id = re.search(r'/expenses/(\d+)/edit', body).group(1)
check('Smoke test expense' in body, 'created expense')
check('Smoke test expense' not in b.get('/expenses'), 'expense isolation')
try:
    b.get(f'/expenses/{expense_id}/edit')
    raise AssertionError('foreign expense is accessible')
except HTTPError as error:
    check(error.code == 404, 'foreign expense status')
body = a.post(f'/expenses/{expense_id}/edit', {'amount': '1500.25', 'category_id': '2', 'description': '=1+1', 'date': '2026-09-16'})
check('1\xa0500,25' in body, 'updated amount')
check('=1+1' not in a.get('/expenses?category=1'), 'category filter')
check('=1+1' in a.get('/expenses?category=2&from=2026-09-16&to=2026-09-16'), 'combined filter')
check(json.loads(a.get('/api/stats/by-category'))[0]['amount'] == 150025, 'category aggregate')
months = json.loads(a.get('/api/stats/by-month?year=2026'))
check(len(months) == 12 and months[8]['amount'] == 150025, 'monthly aggregate')
check(json.loads(b.get('/api/stats/by-category')) == [], 'statistics isolation')
rows = list(csv.reader(io.StringIO(a.get('/expenses/export')), delimiter=';'))
check(len(rows) == 2 and rows[1][2] == "'=1+1" and rows[1][3] == '1500,25', 'CSV output')
check(len(list(csv.reader(io.StringIO(b.get('/expenses/export')), delimiter=';'))) == 1, 'CSV isolation')
body = a.post(f'/expenses/{expense_id}/delete', {})
check('У каждой истории есть начало' in body, 'delete expense')
today = date.today().isoformat()
month = today[:7]
a.get('/budgets')
a.post('/budgets', {'month': month, 'category_id': '1', 'amount': '100'})
body = a.post('/recurring', {'amount': '125', 'category_id': '1', 'description': 'Recurring smoke', 'date': today, 'frequency': 'monthly', 'end_date': ''})
rule_id = re.search(r'/recurring/(\d+)/edit', body).group(1)
body = a.get('/expenses')
generated_id = re.search(r'/expenses/(\d+)/edit', body).group(1)
check('Recurring smoke' in body and 'бюджет превышен на 25,00' in body, 'automatic expense and warning')
check('Превышение на 25,00' in a.get('/budgets'), 'budget total')
check('Recurring smoke' not in b.get('/recurring'), 'schedule isolation')
check('Превышение' not in b.get('/budgets'), 'budget isolation')
try:
    b.get(f'/recurring/{rule_id}/edit')
    raise AssertionError('foreign schedule is accessible')
except HTTPError as error:
    check(error.code == 404, 'foreign schedule status')
a.post(f'/recurring/{rule_id}/pause', {})
a.post(f'/recurring/{rule_id}/resume', {})
a.post(f'/recurring/{rule_id}/edit', {'amount': '150', 'category_id': '1', 'description': 'Recurring smoke updated', 'date': today, 'frequency': 'weekly', 'end_date': ''})
check(json.loads(a.get('/api/stats/by-category'))[0]['amount'] == 12500, 'no duplicates or history rewrites')
a.post('/theme', {'theme': 'dark', 'return_to': '/stats'})
check('data-theme="dark"' in a.get('/stats'), 'dark theme persistence')
a.post('/theme', {'theme': 'system', 'return_to': '/recurring'})
a.post(f'/recurring/{rule_id}/delete', {})
check('Recurring smoke' in a.get('/expenses'), 'history survives schedule deletion')
a.post(f'/expenses/{generated_id}/delete', {})
a.post('/budgets/1/delete', {'month': month})
for client in [a, b]:
    client.post('/logout', {})
    check('Рады видеть вас' in client.get('/expenses'), 'logout')
print(json.dumps({'status': 'PASS', 'base_url': base, 'accounts': emails, 'checks': ['registration', 'login', 'create', 'edit', 'delete', 'filters', 'user isolation', 'category stats', 'monthly stats', 'CSV', 'budgets and warnings', 'recurring schedules', 'pause/resume', 'idempotency', 'dark theme persistence', 'logout']}, ensure_ascii=False))
