function main(data) {
  const result = data.result ?? "";
  console.log(`session=${data.session} task_hash=${data.task_hash} time=${data.time}`);
  return {
    success: 1,
    debug:
      `done in ${data.during_ms ?? 0}ms, result ${result.length} chars, ` +
      `totals in=${data.input_totals ?? 0}(${data.hit_totals ?? 0}%) out=${data.output_totals ?? 0}`,
    error: "",
  };
}

const raw = require("fs").readFileSync(0, "utf8");
const result = main(raw.trim() ? JSON.parse(raw) : {});
console.log(JSON.stringify(result));
