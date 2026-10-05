(function () {
  var header = document.querySelector(".site-header");
  var toggle = document.querySelector(".nav-toggle");
  if (header && toggle) {
    toggle.addEventListener("click", function () {
      var open = header.classList.toggle("is-open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    });
  }

  var layout = document.querySelector("[data-doc-layout]");
  if (layout) {
    var buttons = layout.querySelectorAll("[data-doc-target]");
    var panels = layout.querySelectorAll("[data-doc-panel]");

    function showPanel(id, pushHash) {
      panels.forEach(function (panel) {
        var on = panel.getAttribute("data-doc-panel") === id;
        panel.hidden = !on;
      });
      buttons.forEach(function (btn) {
        var on = btn.getAttribute("data-doc-target") === id;
        if (on) btn.setAttribute("aria-current", "true");
        else btn.removeAttribute("aria-current");
      });
      if (pushHash) {
        history.replaceState(null, "", "#" + id);
      }
    }

    function panelForHash(hash) {
      if (!hash) return "overview";
      var known = Array.prototype.some.call(panels, function (p) {
        return p.getAttribute("data-doc-panel") === hash;
      });
      if (known) return hash;
      if (hash.indexOf("pc-") === 0) return "preset-catalog";
      return null;
    }

    buttons.forEach(function (btn) {
      btn.addEventListener("click", function () {
        showPanel(btn.getAttribute("data-doc-target"), true);
      });
    });

    layout.querySelectorAll(".doc-subnav a[href^='#']").forEach(function (link) {
      link.addEventListener("click", function (ev) {
        var id = (link.getAttribute("href") || "").replace(/^#/, "");
        var panelId = panelForHash(id);
        if (!panelId) return;
        ev.preventDefault();
        showPanel(panelId, false);
        history.replaceState(null, "", "#" + id);
        var target = document.getElementById(id);
        if (target) target.scrollIntoView({ block: "start" });
      });
    });

    var initial = (location.hash || "").replace(/^#/, "");
    var start = panelForHash(initial) || "overview";
    showPanel(start, false);
    if (initial && initial !== start) {
      var el = document.getElementById(initial);
      if (el) el.scrollIntoView({ block: "start" });
    }

    window.addEventListener("hashchange", function () {
      var id = (location.hash || "").replace(/^#/, "");
      var panelId = panelForHash(id);
      if (!panelId) return;
      showPanel(panelId, false);
      if (id !== panelId) {
        var target = document.getElementById(id);
        if (target) target.scrollIntoView({ block: "start" });
      }
    });
  }

  var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var nodes = document.querySelectorAll(".reveal");
  if (reduceMotion || !("IntersectionObserver" in window) || !nodes.length) {
    nodes.forEach(function (el) {
      el.classList.add("is-in");
    });
    return;
  }

  var io = new IntersectionObserver(
    function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add("is-in");
          io.unobserve(entry.target);
        }
      });
    },
    { threshold: 0.12, rootMargin: "0px 0px -8% 0px" }
  );

  nodes.forEach(function (el) {
    io.observe(el);
  });
})();
