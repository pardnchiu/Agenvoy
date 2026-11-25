import json
import sys


def main(data):
    answers = data.get("answers") or []
    print(
        f"session={data.get('session')} task_hash={data.get('task_hash')} time={data.get('time')}"
    )
    answered = sum(1 for item in answers if item.get("answer"))
    lines = [f"answered {answered}/{len(answers)} questions"]
    for item in answers:
        lines.append(f"Q: {item.get('question', '')} A: {item.get('answer', '')}")
    return {"success": 1, "debug": "\n".join(lines), "error": ""}


raw = sys.stdin.read()
result = main(json.loads(raw) if raw.strip() else {})
print(json.dumps(result, ensure_ascii=False))
