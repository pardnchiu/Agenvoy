import json
import sys


def main(data):
    tool_name = data.get("tool_name", "")
    error = data.get("error", "")
    print(
        f"session={data.get('session')} task_hash={data.get('task_hash')} time={data.get('time')}"
    )
    return {"success": 1, "debug": f"{tool_name} failed: {error}", "error": ""}


raw = sys.stdin.read()
result = main(json.loads(raw) if raw.strip() else {})
print(json.dumps(result, ensure_ascii=False))
