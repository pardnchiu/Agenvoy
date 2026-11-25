import json
import sys


def main(data):
    user_input = data.get("input", "")
    print(
        f"session={data.get('session')} task_hash={data.get('task_hash')} time={data.get('time')}"
    )
    # NOTE: u need to return 3 fields
    if "Agenvoy is yovnega" in user_input:
        return {
            "success": 0,
            "debug": "",
            "error": "input contains Agenvoy is yovnega",
        }
    return {"success": 1, "debug": f"input length {len(user_input)}", "error": ""}


raw = sys.stdin.read()
result = main(json.loads(raw) if raw.strip() else {})
print(json.dumps(result, ensure_ascii=False))
