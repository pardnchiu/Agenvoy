const fs = require("fs");
const path = require("path");
const { marked } = require("marked");

const PAGES_DIR = path.join(__dirname, "public/docs/pages");
const OUT_DIR = path.join(__dirname, "public/docs");

const NAV = [
  { section: "Overview", items: [
    { slug: "home", label: "Home" },
    { slug: "getting-started", label: "Getting Started" },
  ]},
  { section: "Concepts", items: [
    { slug: "sessions", label: "Sessions & Agents" },
    { slug: "execution-engine", label: "Execution Engine" },
    { slug: "providers", label: "Providers" },
  ]},
  { section: "User Guide", items: [
    { slug: "cli-commands", label: "CLI Commands" },
    { slug: "tui-guide", label: "TUI Guide" },
    { slug: "rest-api", label: "REST API" },
    { slug: "config-files", label: "Configuration" },
    { slug: "config-integrations", label: "Integration Config" },
  ]},
  { section: "Tools", items: [
    { slug: "built-in-tools", label: "Built-in Tools" },
    { slug: "tool-extension", label: "Tool Extension" },
    { slug: "tool-rules", label: "Tool Design & Rules" },
  ]},
  { section: "Features", items: [
    { slug: "memory-system", label: "Memory System" },
    { slug: "skill-basics", label: "Skill System" },
    { slug: "scheduler-skills", label: "Scheduler & Self-Improvement" },
    { slug: "mcp-server", label: "MCP Server" },
    { slug: "mcp-client", label: "MCP Client" },
    { slug: "kuradb-rag", label: "KuraDB & RAG" },
  ]},
  { section: "Security", items: [
    { slug: "sandbox", label: "Sandbox" },
    { slug: "security", label: "Security Model" },
  ]},
  { section: "Reference", items: [
    { slug: "architecture", label: "Architecture" },
    { slug: "comparison", label: "Comparison" },
  ]},
];

const DESCRIPTIONS = {
  "home": "Agenvoy documentation — a personal AI agent that runs on your machine.",
  "getting-started": "Install Agenvoy and run your first agent session in under 60 seconds.",
  "sessions": "Sessions, agent personas, routing, and per-session concurrency in Agenvoy.",
  "execution-engine": "How the iteration loop, three-pass tool dispatch, and circuit breaker work.",
  "providers": "Supported LLM providers — Claude, OpenAI, Gemini, Codex, and more.",
  "cli-commands": "CLI commands, make shortcuts, input prefixes, and environment variables.",
  "tui-guide": "TUI keyboard shortcuts and slash commands reference.",
  "rest-api": "REST API endpoints for sending messages and managing sessions.",
  "config-files": "Configuration file layout, bot.md format, and permission modes.",
  "config-integrations": "MCP, provider, KuraDB, Telegram, and Discord configuration.",
  "built-in-tools": "60+ built-in tools — file ops, web search, orchestration, memory, and more.",
  "tool-extension": "Auto-generate tools from natural language, or add script/API/MCP tools.",
  "tool-rules": "Tool design guidelines, concurrency markers, timeouts, and credential auto-heal.",
  "memory-system": "Three-tier conversation memory — context window, semantic search, FTS5 archive.",
  "skill-basics": "Loadable markdown skill packs with slash-command and natural-language triggers.",
  "scheduler-skills": "Cron and one-shot scheduling with auto-fix on skill failure.",
  "mcp-server": "Expose your sandboxed tools to Claude Code, Codex, and any MCP-compatible agent.",
  "mcp-client": "Connect to external MCP servers via stdio or HTTP/SSE.",
  "kuradb-rag": "KuraDB child process for keyword and semantic document search.",
  "sandbox": "OS-native sandbox — bubblewrap on Linux, sandbox-exec on macOS.",
  "security": "Permission modes, keychain, system prompt protection, and MCP isolation.",
  "architecture": "System layers, cross-cutting principles, and TUI design choices.",
  "comparison": "How Agenvoy compares to other AI agent platforms.",
};

function slugify(text) {
  return text.toLowerCase().replace(/[^\w\s-]/g, "").replace(/\s+/g, "-").replace(/-+/g, "-").trim();
}

function buildSidebar(activeSlug) {
  let html = "";
  for (const group of NAV) {
    html += `<div class="nav-divider"></div>\n`;
    html += `<div class="nav-section">${group.section}</div>\n`;
    for (const item of group.items) {
      const cls = item.slug === activeSlug ? " active" : "";
      const href = item.slug === "home" ? "/docs/" : `/docs/${item.slug}`;
      html += `<a class="nav-item${cls}" href="${href}">${item.label}</a>\n`;
    }
  }
  return html.replace(/^<div class="nav-divider"><\/div>\n/, "");
}

function buildTOC(html) {
  const headings = [];
  const regex = /<h([23])[^>]*id="([^"]*)"[^>]*>(.*?)<\/h\1>/g;
  let m;
  while ((m = regex.exec(html)) !== null) {
    headings.push({ depth: parseInt(m[1]), id: m[2], text: m[3].replace(/<[^>]+>/g, "") });
  }
  if (!headings.length) return '<div class="toc-title">On this page</div>';
  let toc = '<div class="toc-title">On this page</div>\n';
  for (const h of headings) {
    const cls = h.depth === 3 ? " depth-3" : "";
    toc += `<a class="toc-link${cls}" href="#${h.id}">${h.text}</a>\n`;
  }
  return toc;
}

function addHeadingIds(html) {
  return html.replace(/<h([1-4])>(.*?)<\/h\1>/g, (match, level, text) => {
    const id = slugify(text.replace(/<[^>]+>/g, ""));
    return `<h${level} id="${id}">${text}</h${level}>`;
  });
}

function renderPage(slug, title, description, sidebar, content, toc) {
  const canonical = slug === "home" ? "https://agenvoy.com/docs/" : `https://agenvoy.com/docs/${slug}`;
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>${title} - Agenvoy Docs</title>
    <meta name="description" content="${description}" />
    <link rel="icon" href="/logo-min.svg" type="image/svg+xml" />
    <link rel="canonical" href="${canonical}" />
    <meta property="og:title" content="${title} - Agenvoy Docs" />
    <meta property="og:description" content="${description}" />
    <meta property="og:url" content="${canonical}" />
    <meta property="og:type" content="article" />
    <meta property="og:site_name" content="Agenvoy" />
    <script async src="https://www.googletagmanager.com/gtag/js?id=G-L5VYEZPVXX"></script>
    <script>window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments)}gtag("js",new Date());gtag("config","G-L5VYEZPVXX");</script>
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet" />
    <style>
      :root {
        --bg: #ffffff; --bg-alt: #f8fafc; --surface: #f1f5f9;
        --border: #e2e8f0; --border-light: #cbd5e1;
        --brand: #1461DC; --brand-light: #2563eb;
        --brand-bg: rgba(20,97,220,0.06);
        --text: #0f172a; --text-2: #334155; --muted: #64748b;
        --sidebar-w: 260px; --toc-w: 220px; --header-h: 56px;
      }
      *,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
      html{scroll-behavior:smooth;scroll-padding-top:calc(var(--header-h)+16px)}
      body{font-family:"Inter",system-ui,sans-serif;background:var(--bg);color:var(--text);line-height:1.6;-webkit-font-smoothing:antialiased}
      a{color:var(--brand-light);text-decoration:none}a:hover{text-decoration:underline}
      code,.mono{font-family:"JetBrains Mono",monospace}
      .header{position:fixed;top:0;left:0;right:0;z-index:100;height:var(--header-h);background:rgba(255,255,255,0.9);backdrop-filter:blur(12px);border-bottom:1px solid var(--border);display:flex;align-items:center;padding:0 20px;gap:16px}
      .header-logo{display:flex;align-items:center;gap:8px;font-weight:700;font-size:16px;color:var(--text);flex-shrink:0}.header-logo picture{display:flex;align-items:center}
      .header-logo img{height:30px}
      @media(max-width:480px){.header-logo img{height:24px}}
      .header-sep{width:1px;height:24px;background:var(--border);flex-shrink:0}
      .header-title{font-size:14px;font-weight:500;color:var(--muted)}
      .header-links{margin-left:auto;display:flex;gap:12px}
      .header-links a{font-size:13px;font-weight:500;color:var(--muted);padding:4px 10px;border-radius:5px;transition:color .15s,background .15s}
      .header-links a:hover{color:var(--text);background:var(--surface);text-decoration:none}
      .mobile-menu-btn{display:none;background:none;border:none;font-size:20px;cursor:pointer;color:var(--text);padding:4px}
      .layout{display:grid;grid-template-columns:var(--sidebar-w) 1fr var(--toc-w);margin-top:var(--header-h);min-height:calc(100vh - var(--header-h))}
      .sidebar{position:sticky;top:var(--header-h);height:calc(100vh - var(--header-h));overflow-y:auto;padding:16px 0;border-right:1px solid var(--border);background:var(--bg)}
      .sidebar::-webkit-scrollbar{width:4px}.sidebar::-webkit-scrollbar-thumb{background:var(--border);border-radius:2px}
      .nav-section{padding:10px 20px 4px;font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.06em;color:var(--muted)}
      .nav-item{display:block;padding:6px 20px;font-size:13.5px;font-weight:450;color:var(--text-2);cursor:pointer;border-left:3px solid transparent;transition:background .1s,border-color .1s,color .1s}
      .nav-item:hover{background:var(--brand-bg);color:var(--text);text-decoration:none}
      .nav-item.active{color:var(--brand);border-left-color:var(--brand);background:var(--brand-bg);font-weight:600}
      .nav-divider{height:1px;background:var(--border);margin:8px 16px}
      .content{padding:32px 48px 80px;max-width:100%;min-width:0;overflow:hidden}
      .content h1{font-size:30px;font-weight:700;letter-spacing:-.02em;margin-bottom:8px;padding-bottom:12px;border-bottom:1px solid var(--border)}
      .content h2{font-size:22px;font-weight:600;margin-top:36px;margin-bottom:12px;padding-bottom:6px;border-bottom:1px solid var(--border);letter-spacing:-.01em}
      .content h3{font-size:17px;font-weight:600;margin-top:28px;margin-bottom:8px}
      .content h4{font-size:15px;font-weight:600;margin-top:20px;margin-bottom:6px}
      .content p{margin-bottom:12px;font-size:15px;color:var(--text-2)}
      .content ul,.content ol{margin-bottom:12px;padding-left:24px;font-size:15px;color:var(--text-2)}
      .content li{margin-bottom:4px}.content li>ul,.content li>ol{margin-top:4px;margin-bottom:4px}
      .content blockquote{border-left:3px solid var(--brand);padding:8px 16px;margin-bottom:12px;background:var(--brand-bg);border-radius:0 6px 6px 0;font-size:14px;color:var(--text-2)}
      .content blockquote p{margin-bottom:4px}
      .content code{background:var(--surface);padding:2px 6px;border-radius:4px;font-size:13px;color:#c7254e}
      .content pre{background:#1e293b;border:1px solid #334155;border-radius:8px;padding:16px;overflow-x:auto;margin-bottom:16px}
      .content pre code{background:none;padding:0;color:#e2e8f0;font-size:13px;line-height:1.6}
      .content table{width:100%;border-collapse:collapse;margin-bottom:16px;font-size:14px}
      .content th,.content td{padding:8px 12px;text-align:left;border:1px solid var(--border)}
      .content th{background:var(--surface);font-weight:600;font-size:13px}
      .content td{color:var(--text-2)}
      .content hr{border:none;border-top:1px solid var(--border);margin:24px 0}
      .content a{color:var(--brand-light)}.content img{max-width:100%;border-radius:6px}.content strong{color:var(--text)}
      .toc{position:sticky;top:var(--header-h);height:calc(100vh - var(--header-h));overflow-y:auto;padding:20px 16px;border-left:1px solid var(--border);background:var(--bg)}
      .toc::-webkit-scrollbar{width:4px}.toc::-webkit-scrollbar-thumb{background:var(--border);border-radius:2px}
      .toc-title{font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.06em;color:var(--muted);margin-bottom:10px}
      .toc-link{display:block;padding:3px 0;font-size:12.5px;color:var(--muted);transition:color .15s;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
      .toc-link:hover{color:var(--text);text-decoration:none}
      .toc-link.depth-3{padding-left:12px;font-size:12px}
      @media(max-width:1100px){.layout{grid-template-columns:var(--sidebar-w) 1fr}.toc{display:none}}
      @media(max-width:768px){.mobile-menu-btn{display:block}.header-links{display:none}.layout{grid-template-columns:1fr}.sidebar{display:none;position:fixed;top:var(--header-h);left:0;width:280px;z-index:50;box-shadow:4px 0 12px rgba(0,0,0,.08)}.sidebar.open{display:block}.toc{display:none}.content{padding:24px 20px 60px}}
    </style>
  </head>
  <body>
    <header class="header">
      <button class="mobile-menu-btn" onclick="document.querySelector('.sidebar').classList.toggle('open')" aria-label="Menu">&#9776;</button>
      <a href="/" class="header-logo"><picture><source media="(max-width: 480px)" srcset="/logo-min.svg" /><img src="/logo-text.svg" alt="Agenvoy" /></picture></a>
      <span class="header-sep"></span>
      <span class="header-title">Documentation</span>
      <div class="header-links">
        <a href="/">Home</a>
        <a href="https://github.com/pardnchiu/agenvoy" target="_blank" rel="noopener">GitHub</a>
      </div>
    </header>
    <div class="layout">
      <nav class="sidebar">${sidebar}</nav>
      <main class="content">${content}</main>
      <aside class="toc">${toc}</aside>
    </div>
    <script>
      document.querySelectorAll('.sidebar .nav-item').forEach(function(el){
        el.addEventListener('click',function(){document.querySelector('.sidebar').classList.remove('open')})
      });
      var tocObs=new IntersectionObserver(function(entries){
        entries.forEach(function(e){
          if(e.isIntersecting){
            document.querySelectorAll('.toc-link').forEach(function(l){
              l.classList.toggle('active',l.getAttribute('href')==='#'+e.target.id)
            })
          }
        })
      },{rootMargin:'-80px 0px -70% 0px'});
      document.querySelectorAll('.content h2,.content h3').forEach(function(h){tocObs.observe(h)});
    </script>
  </body>
</html>`;
}

marked.setOptions({ gfm: true, breaks: false });

const allSlugs = NAV.flatMap(g => g.items.map(i => i.slug));
let built = 0;

for (const slug of allSlugs) {
  const mdPath = path.join(PAGES_DIR, `${slug}.md`);
  if (!fs.existsSync(mdPath)) {
    console.warn(`SKIP: ${slug}.md not found`);
    continue;
  }

  const md = fs.readFileSync(mdPath, "utf-8");
  let html = marked.parse(md);
  html = addHeadingIds(html);

  const label = NAV.flatMap(g => g.items).find(i => i.slug === slug)?.label || slug;
  const desc = DESCRIPTIONS[slug] || `${label} — Agenvoy documentation.`;
  const sidebar = buildSidebar(slug);
  const toc = buildTOC(html);
  const page = renderPage(slug, label, desc, sidebar, html, toc);

  const outPath = slug === "home"
    ? path.join(OUT_DIR, "index.html")
    : path.join(OUT_DIR, `${slug}.html`);

  fs.writeFileSync(outPath, page);
  built++;
  console.log(`OK: ${outPath}`);
}

// Generate sitemap.xml
const today = new Date().toISOString().split("T")[0];
let sitemap = `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n`;
sitemap += `  <url><loc>https://agenvoy.com/</loc><changefreq>weekly</changefreq><priority>1.0</priority><lastmod>${today}</lastmod></url>\n`;
sitemap += `  <url><loc>https://agenvoy.com/docs/</loc><changefreq>weekly</changefreq><priority>0.9</priority><lastmod>${today}</lastmod></url>\n`;
for (const slug of allSlugs) {
  if (slug === "home") continue;
  const mdPath = path.join(PAGES_DIR, `${slug}.md`);
  if (!fs.existsSync(mdPath)) continue;
  sitemap += `  <url><loc>https://agenvoy.com/docs/${slug}</loc><changefreq>monthly</changefreq><priority>0.7</priority><lastmod>${today}</lastmod></url>\n`;
}
sitemap += `</urlset>\n`;
fs.writeFileSync(path.join(__dirname, "public/sitemap.xml"), sitemap);
console.log(`OK: sitemap.xml (${allSlugs.length + 1} URLs)`);

console.log(`\nBuilt ${built} pages.`);
