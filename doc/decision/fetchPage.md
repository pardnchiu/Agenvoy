# fetch_page: Before and After the Decision Model

Last updated: 2026-11-19

> Back to [Documentation](../doc.md)

This document compares measured results of `fetch_page` before (old) and after (new) decision-model filtering was added. It focuses on three questions:

1. **Relevance**: are pages unrelated to the task excluded?
2. **Content filtering**: are unrelated parts removed before returning, while the key content survives?
3. **Reduction**: how much less content actually reaches the agent?

## Summary

| Metric | Old | New |
| --- | --- | --- |
| Correct page-relevance decisions | 19 / 22 | **22 / 22** |
| Key content kept for related tasks | 11 / 15 (plus 1 JSON cut into invalid JSON) | **15 / 15** |
| Content sent for related tasks (unfiltered 3,130 KB) | 595 KB, 81.0% less | **320 KB, 89.8% less** |
| Content sent for unrelated tasks | 104 KB | **2.2 KB** (notice only) |
| Average time: regular pages | 5.14 s | 5.42 s |
| Average time: pages over 1 MiB | 15.2 s | 20.7 s |

Most of the old version's reduction comes from a fixed 100 KiB cut and from hiding related pages it misjudged as unrelated, so it is paid for with lost information. The new version's reduction comes from dropping blocks unrelated to the task, and every key item is kept.

## How the two versions work

| | Old | New |
| --- | --- | --- |
| Stored in cache | Result after noise removal and truncation | Original content with only noise (ads, navigation and the like) removed |
| Long pages | Cut at 100 KiB | Filtered block by block against the task; only related blocks are returned |
| Page relevance | Whole page judged from its opening only; markdown only | Short pages judged as a whole; long pages judged block by block and excluded only when no block is related; same rules for markdown and json |
| Over 1 MiB | Cut at 100 KiB | The first 1 MiB is checked and only blocks that directly answer the task are kept; if none, the agent is told the site is too large and to download it |
| Error pages | Detected by title | Detected by the actual HTTP status and the title |

In the new version, task-based filtering runs right before returning, so freshly fetched content and content read from the cache go through the same path. The same page fetched for different tasks is filtered separately for each.

## Cache and re-filtering

The new version separates what is stored from what is returned:

1. **The cache stores the original content**: after fetching, the decision model removes only noise such as ads and navigation (markdown, `keep_links` off, content up to 1 MiB), and the rest is stored in ToriiDB as is. The cache never holds task-filtered results or withheld notices.
2. **No cache entry**: the page is fetched and stored as in point 1, then filtered from that original content against the current task by the decision model and returned.
3. **Cache hit**: when the same URL (same parameters) is requested again, the original content is read from the cache and filtered again against the current task by the decision model before returning.

So when different tasks request the same URL, none of them reuses the previous task's filtering. A page withheld as unrelated for one task is still available to the next related task.

Measured (English Wikipedia: Taiwan, two tasks in a row in one session):

| Format | Task | Source | Result | Time |
| --- | --- | --- | --- | --- |
| markdown | how to bake sourdough bread | fetch | withheld, 166 B | 7.6 s |
| markdown | capital of Taiwan | cache | filtered to 45 KB, includes `Taipei` | 0.7 s |
| json | how to bake sourdough bread | fetch | withheld, 160 B | 7.2 s |
| json | capital of Taiwan | cache | filtered to 113 KB, includes `Taipei`, valid JSON | 1.4 s |

After both calls the cache still holds the full original page (markdown 229 KB, json 572 KB). Reading from the cache saves the fetch; the remaining time is the task-based re-filtering.

## 1. Relevance: unrelated pages are excluded

Each page is paired with one related task and one unrelated task. A related task should not be hidden; an unrelated task should get a withheld notice.

| Page | Related: old | Related: new | Unrelated: old | Unrelated: new |
| --- | --- | --- | --- | --- |
| MDN Array.map | returned | returned | excluded | excluded |
| Python stdtypes | **wrongly hidden** | returned | excluded | excluded |
| pkg.go.dev net/http | returned | returned | excluded | excluded |
| English Wikipedia: Taiwan | returned | returned | excluded | excluded |
| Chinese Wikipedia: Japan | returned | returned | excluded | excluded |
| Effective Go | **wrongly hidden** | returned | excluded | excluded |
| RFC 9112 | returned | returned | excluded | excluded |
| ECMAScript (1.8 MB) | returned | returned | excluded | excluded |
| example.com | returned | returned | excluded | excluded |
| GitHub go-rod | returned | returned | excluded | excluded |
| WWII (json) | returned | returned | **not excluded (102 KB)** | excluded |

- **Old version hid related pages**: it judged the whole page from its opening. The `maxsplit` part of Python stdtypes and the `defer` part of Effective Go are not near the top, so both pages were hidden and the agent had to look elsewhere.
- **Old version let an unrelated page through**: it never judged json relevance, so an unrelated task still received 102 KB of WWII content.

## 2. Content filtering: unrelated parts are removed before returning

Related tasks only. Each task lists 1-2 items that must appear in the result to count as kept.

| Page / task | Key content | Old | New |
| --- | --- | --- | --- |
| MDN Array.map / what map returns | `new array`, `callbackFn` | 2/2 full page | 2/2 full page |
| Python stdtypes / maxsplit | `maxsplit`, `sep` | **0/2 hidden** | 2/2 filtered |
| net/http / Client timeout | `Timeout`, `Client` | 2/2 full page | 2/2 filtered |
| English Wikipedia: Taiwan / capital | `Taipei` | 1/1 truncated | 1/1 filtered |
| Chinese Wikipedia: Japan / capital | `東京` | 1/1 truncated | 1/1 filtered |
| Effective Go / defer | `defer`, `LIFO` | **0/2 hidden** | 2/2 filtered |
| RFC 9112 / chunked format | `chunk-size`, `last-chunk` | 2/2 full page | 2/2 filtered |
| ECMAScript / Number type | `Number type` | 1/1 opening only | 1/1 answer blocks only |
| example.com / purpose | `documentation` | 1/1 full page | 1/1 full page |
| GitHub go-rod / what rod is | `DevTools Protocol` | 1/1 full page | 1/1 full page |
| WWII json / invasion of Poland | `1 September 1939` | **1/1 but invalid JSON** | 1/1 valid JSON |

- The new version keeps all 15 key items. 8 long pages were filtered against the task; 3 short pages were short enough to return whole.
- The old version did no task-based filtering, only a fixed 100 KiB cut. When the answer sits further down it is lost; the 2 wrongly hidden pages lost 4 key items.
- The old ECMAScript result contains the words `Number type`, but only as a table-of-contents mention near the top; the new version's 1.3 KB is the definition itself.

## 3. How much content is removed

The baseline is the unfiltered `fetch_page` output, which is what the cache stores (frontmatter plus the body after noise removal). ECMAScript is over 1 MiB, so its full page size is the baseline. Related tasks only; for unrelated tasks the new version always returns a notice of about 150-270 B.

| Page | Unfiltered | Old | Old reduction | New | New reduction |
| --- | --- | --- | --- | --- | --- |
| MDN Array.map | 5.4 KB | 5.4 KB | 0.5% more | 5.4 KB | 0% |
| Python stdtypes | 169 KB | 185 B | 99.9% (hidden) | 9.0 KB | 94.6% |
| net/http | 84 KB | 84 KB | 0% | 17 KB | 79.6% |
| English Wikipedia: Taiwan | 224 KB | 100 KB | 55.4% (truncated) | 42 KB | 81.1% |
| Chinese Wikipedia: Japan | 146 KB | 100 KB | 31.6% (truncated) | 19 KB | 87.1% |
| Effective Go | 96 KB | 197 B | 99.8% (hidden) | 13 KB | 86.9% |
| RFC 9112 | 88 KB | 88 KB | 0% | 35 KB | 60.2% |
| ECMAScript | 1,806 KB | 100 KB | 94.5% (truncated) | 1.2 KB | 99.9% |
| example.com | 1.5 KB | 1.5 KB | 1.7% more | 1.5 KB | 0% |
| GitHub go-rod | 2.2 KB | 2.1 KB | 2.9% | 2.1 KB | 4.1% |
| WWII json | 435 KB | 100 KB | 77.0% (invalid JSON) | 167 KB | 61.5% |
| **Total** | **3,130 KB** | **595 KB** | **81.0%** | **320 KB** | **89.8%** |

- At roughly 4 bytes per token: unfiltered is about 780K tokens, new about 80K, old about 150K.
- MDN and example.com are already short and need no filtering, so the new reduction is 0%. The 4.1% for go-rod comes from noise removal differing slightly between two fetches. Old short pages come out 1-2% larger because of the `Web page content:` wrapper.
- WWII json is still 167 KB, the largest new output. json cannot carry a filter notice, so the agent cannot tell it was filtered.

## Timing

| Scope | Old average | New average | Difference |
| --- | --- | --- | --- |
| Regular pages (20 cases, excluding ECMAScript) | 5.14 s | 5.42 s | +0.28 s |
| Over 1 MiB (2 ECMAScript cases) | 15.2 s | 20.7 s | +5.5 s |

Times include fetching in Chrome (every call used `force` and skipped the cache), so most of the time is the fetch itself. The extra time in the new version is block-by-block judging against the task; pages over 1 MiB cost the most because the first 1 MiB is judged in small batches.

## Method and limitations

- **Old**: git `b45377ab`, dependencies are the go.mod releases (go-browser v0.3.4).
- **New**: working tree, using local go-browser (with the KeepLinks fix, since released as v0.3.5), go-llm-router and go-pkg through go.work.
- Both versions were called through the tool entry `toolregister.Dispatch("fetch_page")` with `force: true`; decision model `typesafe@jev-latest`; run one after the other so they did not compete for Chrome or the decision model.
- Each case ran once. The decision model's answers can vary between runs, so borderline blocks may come and go.
- Key content was checked by string match, which shows the content is present but not that answers are good. The sample is 11 pages, weighted toward English technical documentation.
- The reduction baseline is the output after readability extraction and noise removal, not the page's raw HTML. It was fetched separately, so noise removal may differ slightly from the test run.
