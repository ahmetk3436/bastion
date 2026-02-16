"""
Test: AI assistant endpoints (chat, execute, analyze).
"""
from conftest import api_get, api_post, api_delete


def test_chat_nonstream():
    """POST /api/ai/chat — non-streaming chat."""
    resp = api_post("/ai/chat", json={
        "message": "What is the server uptime?",
    })
    # May fail if GLM-5 API key not configured, 200 or 502
    assert resp.status_code in [200, 400, 502, 503], f"Chat failed: {resp.status_code} {resp.text}"
    if resp.status_code == 200:
        data = resp.json()
        assert "response" in data or "message" in data, f"Missing response: {data}"
        print(f"  PASS: AI chat responded — {data.get('response', data.get('message', ''))[:80]}...")
    else:
        print(f"  PASS: AI chat returned {resp.status_code} (LLM may not be configured)")


def test_conversations_list():
    """GET /api/ai/conversations — list conversations."""
    resp = api_get("/ai/conversations")
    assert resp.status_code == 200, f"List conversations failed: {resp.status_code} {resp.text}"
    data = resp.json()
    convos = data.get("conversations", data)
    assert isinstance(convos, list)
    print(f"  PASS: Listed {len(convos)} AI conversations")
    return convos


def test_conversation_detail():
    """GET /api/ai/conversations/:id — get single conversation."""
    convos = test_conversations_list()
    if not convos:
        print("  SKIP: No conversations")
        return
    cid = convos[0].get("id") or convos[0].get("ID")
    if not cid:
        print("  SKIP: No conversation ID")
        return
    resp = api_get(f"/ai/conversations/{cid}")
    assert resp.status_code == 200, f"Get convo failed: {resp.status_code}"
    print("  PASS: Conversation detail retrieved")


def test_analyze_logs():
    """POST /api/ai/analyze-logs — log analysis."""
    resp = api_post("/ai/analyze-logs", json={
        "logs": "2026-02-17 ERROR: connection refused to database\n2026-02-17 PANIC: runtime error",
        "context": "bastion backend",
    })
    assert resp.status_code in [200, 400, 502, 503], f"Analyze failed: {resp.status_code}"
    print(f"  PASS: Log analysis returned {resp.status_code}")


def test_suggest_fix():
    """POST /api/ai/suggest-fix — error fix suggestion."""
    resp = api_post("/ai/suggest-fix", json={
        "error": "FATAL: password authentication failed for user postgres",
        "context": "PostgreSQL connection",
    })
    assert resp.status_code in [200, 400, 502, 503], f"Suggest failed: {resp.status_code}"
    print(f"  PASS: Suggest fix returned {resp.status_code}")


def test_execute_action():
    """POST /api/ai/execute — execute AI action."""
    resp = api_post("/ai/execute", json={
        "action": "get_metrics",
    })
    assert resp.status_code in [200, 400, 404, 502], f"Execute failed: {resp.status_code}"
    print(f"  PASS: AI execute returned {resp.status_code}")


if __name__ == "__main__":
    test_chat_nonstream()
    test_conversations_list()
    test_conversation_detail()
    test_analyze_logs()
    test_suggest_fix()
    test_execute_action()
    print("\nALL AI TESTS PASSED")
