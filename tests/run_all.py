#!/usr/bin/env python3
"""
Bastion Backend — Run ALL test suites.
Usage: python3 run_all.py
"""
import subprocess
import sys
import os

TESTS_DIR = os.path.dirname(os.path.abspath(__file__))

TEST_FILES = [
    "test_health.py",
    "test_auth.py",
    "test_dashboard.py",
    "test_servers.py",
    "test_commands.py",
    "test_crons.py",
    "test_docker.py",
    "test_monitors.py",
    "test_alerts.py",
    "test_database.py",
    "test_files.py",
    "test_processes.py",
    "test_coolify.py",
    "test_ops.py",
    "test_ai.py",
    "test_audit.py",
]

results = {}
total_pass = 0
total_fail = 0

print("=" * 60)
print("  BASTION BACKEND — FULL TEST SUITE")
print("=" * 60)
print()

for tf in TEST_FILES:
    path = os.path.join(TESTS_DIR, tf)
    if not os.path.exists(path):
        print(f"  SKIP: {tf} (not found)")
        continue

    print(f"--- Running {tf} ---")
    result = subprocess.run(
        [sys.executable, path],
        cwd=TESTS_DIR,
        capture_output=True,
        text=True,
        timeout=120,
    )

    if result.returncode == 0:
        results[tf] = "PASS"
        total_pass += 1
        print(result.stdout)
    else:
        results[tf] = "FAIL"
        total_fail += 1
        print(result.stdout)
        if result.stderr:
            print(f"  STDERR: {result.stderr[:500]}")
    print()

print("=" * 60)
print("  RESULTS SUMMARY")
print("=" * 60)
for tf, status in results.items():
    icon = "✓" if status == "PASS" else "✗"
    print(f"  {icon} {tf}: {status}")

print()
print(f"  Total: {total_pass + total_fail} suites | Pass: {total_pass} | Fail: {total_fail}")
print("=" * 60)

sys.exit(0 if total_fail == 0 else 1)
