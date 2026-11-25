const MAX_INPUT_TOTALS = 2_000_000;

function main(data) {
  const toolName = data.tool_name ?? "";
  const inputTotals = data.input_totals ?? 0;
  const outputTotals = data.output_totals ?? 0;
  let toolArgs = {};
  try {
    toolArgs = JSON.parse(data.tool_args || "{}");
  } catch {}
  console.log(`session=${data.session} task_hash=${data.task_hash} time=${data.time}`);
  if (inputTotals > MAX_INPUT_TOTALS) {
    return { success: 0, debug: "", error: `input tokens ${inputTotals} exceed ${MAX_INPUT_TOTALS}` };
  }
  return {
    success: 1,
    debug: `call ${toolName}(${Object.keys(toolArgs).join(", ")}), totals in=${inputTotals}(${data.hit_totals ?? 0}%) out=${outputTotals}`,
    error: "",
  };
}

const raw = require("fs").readFileSync(0, "utf8");
const result = main(raw.trim() ? JSON.parse(raw) : {});
console.log(JSON.stringify(result));
