import json
import sys


def main(data):
    print(
        f"session={data.get('session')} task_hash={data.get('task_hash')} time={data.get('time')}"
    )
    return {
        "success": 1,
        "debug": (
            f"canceled: {data.get('error', '')}, "
            f"totals in={data.get('input_totals', 0)}({data.get('hit_totals', 0)}%) out={data.get('output_totals', 0)}"
        ),
        "error": "",
    }


raw = sys.stdin.read()
result = main(json.loads(raw) if raw.strip() else {})
print(json.dumps(result, ensure_ascii=False))
