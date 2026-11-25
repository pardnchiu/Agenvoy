import json
import sys


def main(data):
    tool_name = data.get("tool_name", "")
    tool_result = data.get("tool_result", "")
    print(
        f"session={data.get('session')} task_hash={data.get('task_hash')} time={data.get('time')}"
    )
    return {
        "success": 1,
        "debug": (
            f"{tool_name} returned {len(tool_result)} chars in {data.get('during_ms', 0)}ms, "
            f"tokens in={data.get('input_tokens', 0)}({data.get('hit', 0)}%) out={data.get('output_tokens', 0)}, "
            f"totals in={data.get('input_totals', 0)}({data.get('hit_totals', 0)}%) out={data.get('output_totals', 0)}"
        ),
        "error": "",
    }


raw = sys.stdin.read()
result = main(json.loads(raw) if raw.strip() else {})
print(json.dumps(result, ensure_ascii=False))
