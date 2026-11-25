function main(data) {
  const answers = data.answers ?? [];
  console.log(`session=${data.session} task_hash=${data.task_hash} time=${data.time}`);
  const answered = answers.filter((item) => item.answer).length;
  const lines = [`answered ${answered}/${answers.length} questions`];
  for (const item of answers) {
    lines.push(`Q: ${item.question ?? ""} A: ${item.answer ?? ""}`);
  }
  return { success: 1, debug: lines.join("\n"), error: "" };
}

const raw = require("fs").readFileSync(0, "utf8");
const result = main(raw.trim() ? JSON.parse(raw) : {});
console.log(JSON.stringify(result));
