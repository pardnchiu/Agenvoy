import json
import sys


def main(data):
    result = data.get("result", "")
    print(
        f"session={data.get('session')} task_hash={data.get('task_hash')} time={data.get('time')}"
    )
    return {
        "success": 1,
        "debug": (
            f"done in {data.get('during_ms', 0)}ms, result {len(result)} chars, "
            f"totals in={data.get('input_totals', 0)}({data.get('hit_totals', 0)}%) out={data.get('output_totals', 0)}"
        ),
        "error": "",
    }


raw = sys.stdin.read()
result = main(json.loads(raw) if raw.strip() else {})
print(json.dumps(result, ensure_ascii=False))
