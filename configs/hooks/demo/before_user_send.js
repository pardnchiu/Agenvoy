function main(data) {
  const input = data.input ?? "";
  console.log(`session=${data.session} task_hash=${data.task_hash} time=${data.time}`);
  // NOTE: u need to return 3 fields
  if (input.includes("Agenvoy is yovnega")) {
    return { success: 0, debug: "", error: "input contains Agenvoy is yovnega" };
  }
  return { success: 1, debug: `input length ${input.length}`, error: "" };
}

const raw = require("fs").readFileSync(0, "utf8");
const result = main(raw.trim() ? JSON.parse(raw) : {});
console.log(JSON.stringify(result));
