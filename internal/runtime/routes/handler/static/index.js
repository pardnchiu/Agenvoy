(function () {
  var sid = "jarvis";
  var frame = document.getElementById("frame");
  var stBar = document.getElementById("status");
  var stTxt = document.getElementById("stxt");
  var panel = document.getElementById("input-panel");
  var toggle = document.getElementById("input-panel-toggle");
  var input = document.getElementById("input-panel-input");
  var sendBtn = document.getElementById("input-panel-send");
  var busy = false;
  var reader = null;

  function getTS() {
    return new URLSearchParams(window.location.search).get("ts") || "";
  }

  function loadPage(ts) {
    if (ts) {
      frame.src = "/jarvis/page?ts=" + ts;
      history.pushState(null, "", "/jarvis?ts=" + ts);
    } else {
      frame.src = "/jarvis/page";
      history.pushState(null, "", "/jarvis");
    }
  }

  var initTS = getTS();
  frame.src = initTS ? "/jarvis/page?ts=" + initTS : "/jarvis/page";

  window.addEventListener("popstate", function () {
    var ts = getTS();
    frame.src = ts ? "/jarvis/page?ts=" + ts : "/jarvis/page";
  });

  function patchFrameBg() {
    try {
      var d = frame.contentDocument;
      if (d && d.documentElement) {
        d.documentElement.style.minHeight = "100vh";
        d.documentElement.style.background = "#06090d";
      }
      if (d && d.body) {
        d.body.style.minHeight = "100vh";
        if (!d.body.style.background && !d.body.style.backgroundColor) d.body.style.background = "#06090d";
      }
    } catch (e) {}
  }
  frame.addEventListener("load", patchFrameBg);

  toggle.addEventListener("click", function () {
    if (panel.dataset.open === "1") {
      panel.dataset.open = "0";
    } else {
      panel.dataset.open = "1";
      input.focus();
    }
  });

  sendBtn.addEventListener("click", function () {
    var value = input.value.trim();
    if (value) {
      doSend(value);
      input.value = "";
      input.nextElementSibling.innerHTML = "";
      panel.dataset.open = "0";
    } else {
      panel.dataset.open = "0";
    }
  });

  var es = new EventSource("/jarvis/listener");
  es.onmessage = function (e) {
    try {
      var d = JSON.parse(e.data);
      if (d.ts) {
        loadPage(d.ts);
      }
    } catch (err) {}
    if (busy) {
      if (reader) {
        reader.cancel();
        reader = null;
      }
      setStatus("EventDone", "");
    }
  };

  var labels = {
    EventAgentSelect: "selecting agent…",
    EventAgentResult: "agent ready",
    EventSkillResult: "skill matched",
    EventToolCall: "calling tool",
    EventToolCallStart: "running tool",
    EventToolCallEnd: "tool done",
    EventToolResult: "processing result",
    EventText: "generating…",
    EventTextDone: "rendering…",
    EventDone: "done",
    EventError: "error",
    EventExecError: "tool error",
  };

  function setStatus(type, detail) {
    var s = labels[type] || type;
    if (detail) s += ": " + detail;
    stTxt.textContent = s;
    if (type === "EventDone" || type === "EventError") {
      stBar.className = "";
      busy = false;
    } else {
      stBar.className = "active";
    }
  }

  function streamSSE(body) {
    reader = body.getReader();
    var dec = new TextDecoder();
    var buf = "";
    function pump() {
      reader
        .read()
        .then(function (r) {
          if (r.done) {
            if (busy) { busy = false; }
            return;
          }
          buf += dec.decode(r.value, { stream: true });
          var lines = buf.split("\n");
          buf = lines.pop();
          for (var i = 0; i < lines.length; i++) {
            var ln = lines[i];
            if (ln.indexOf("data: ") !== 0) continue;
            try {
              var ev = JSON.parse(ln.slice(6));
              var detail = "";
              if (ev.tool_name) detail = ev.tool_name;
              else if (ev.text && ev.text.length < 60) detail = ev.text;
              setStatus(ev.type, detail);
            } catch (e) {}
          }
          pump();
        })
        .catch(function () {
          if (busy) { busy = false; }
        });
    }
    pump();
  }

  function doSend(v) {
    busy = true;
    setStatus("sending", "");
    stBar.className = "active";
    fetch("/v1/send", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content: v, session_id: sid, sse: true, web_mode: true, persist: true }),
    })
      .then(function (resp) {
        if (!resp.ok) {
          setStatus("EventError", "HTTP " + resp.status);
          return;
        }
        if (resp.body) streamSSE(resp.body);
      })
      .catch(function (e) {
        setStatus("EventError", e.message);
      });
  }

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      if (panel.dataset.open === "1") {
        panel.dataset.open = "0";
        return;
      }
      if (busy) return;
      fetch("/jarvis/reset", { method: "POST" }).then(function () {
        loadPage("");
        setStatus("idle", "");
        stBar.className = "";
      });
    }
  });
})();
