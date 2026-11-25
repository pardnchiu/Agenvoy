import json
import sys

MAX_INPUT_TOTALS = 2_000_000


def main(data):
    tool_name = data.get("tool_name", "")
    input_totals = data.get("input_totals", 0)
    output_totals = data.get("output_totals", 0)
    try:
        tool_args = json.loads(data.get("tool_args") or "{}")
    except json.JSONDecodeError:
        tool_args = {}
    print(
        f"session={data.get('session')} task_hash={data.get('task_hash')} time={data.get('time')}"
    )
    if input_totals > MAX_INPUT_TOTALS:
        return {
            "success": 0,
            "debug": "",
            "error": f"input tokens {input_totals} exceed {MAX_INPUT_TOTALS}",
        }
    return {
        "success": 1,
        "debug": f"call {tool_name}({', '.join(tool_args)}), totals in={input_totals}({data.get('hit_totals', 0)}%) out={output_totals}",
        "error": "",
    }


raw = sys.stdin.read()
result = main(json.loads(raw) if raw.strip() else {})
print(json.dumps(result, ensure_ascii=False))
