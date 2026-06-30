const https = require("https");
const fs = require("fs");
const path = require("path");

const TAGS_DIR = path.join(__dirname, "public/docs/tags");
const REPO = "pardnchiu/Agenvoy";

function fetch(url) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { "User-Agent": "agenvoy-sync", Accept: "application/vnd.github+json" } }, res => {
      let body = "";
      res.on("data", c => body += c);
      res.on("end", () => {
        if (res.statusCode >= 400) return reject(new Error(`HTTP ${res.statusCode}: ${body.slice(0, 200)}`));
        const link = res.headers.link || "";
        const next = (link.match(/<([^>]+)>;\s*rel="next"/) || [])[1] || null;
        resolve({ data: JSON.parse(body), next });
      });
      res.on("error", reject);
    }).on("error", reject);
  });
}

async function fetchAll() {
  const releases = [];
  let url = `https://api.github.com/repos/${REPO}/releases?per_page=100`;
  while (url) {
    const res = await fetch(url);
    releases.push(...res.data);
    url = res.next;
  }
  return releases;
}

async function main() {
  const releases = await fetchAll();
  if (!releases.length) {
    console.log("No releases found");
    process.exit(0);
  }

  fs.mkdirSync(TAGS_DIR, { recursive: true });

  const manifest = {};

  for (const r of releases) {
    const tag = r.tag_name;
    const date = (r.published_at || "").split("T")[0];
    const content = (r.body || "").replace(/\r\n/g, "\n").replace(/\r/g, "\n");
    fs.writeFileSync(path.join(TAGS_DIR, `${tag}.md`), content);
    manifest[tag] = date;
    console.log(`OK: ${tag}.md (${date})`);
  }

  fs.writeFileSync(path.join(TAGS_DIR, "manifest.json"), JSON.stringify(manifest, null, 2));
  console.log(`\nSynced ${releases.length} releases. Latest: ${releases[0].tag_name}`);
}

main().catch(err => { console.error(err); process.exit(1); });
