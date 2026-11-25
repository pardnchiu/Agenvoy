function main(data) {
  const toolName = data.tool_name ?? "";
  const error = data.error ?? "";
  console.log(`session=${data.session} task_hash=${data.task_hash} time=${data.time}`);
  return { success: 1, debug: `${toolName} failed: ${error}`, error: "" };
}

const raw = require("fs").readFileSync(0, "utf8");
const result = main(raw.trim() ? JSON.parse(raw) : {});
console.log(JSON.stringify(result));
