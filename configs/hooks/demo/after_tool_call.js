function main(data) {
  const toolName = data.tool_name ?? "";
  const toolResult = data.tool_result ?? "";
  console.log(`session=${data.session} task_hash=${data.task_hash} time=${data.time}`);
  return {
    success: 1,
    debug:
      `${toolName} returned ${toolResult.length} chars in ${data.during_ms ?? 0}ms, ` +
      `tokens in=${data.input_tokens ?? 0}(${data.hit ?? 0}%) out=${data.output_tokens ?? 0}, ` +
      `totals in=${data.input_totals ?? 0}(${data.hit_totals ?? 0}%) out=${data.output_totals ?? 0}`,
    error: "",
  };
}

const raw = require("fs").readFileSync(0, "utf8");
const result = main(raw.trim() ? JSON.parse(raw) : {});
console.log(JSON.stringify(result));
