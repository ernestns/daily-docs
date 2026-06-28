package main

import "html/template"

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>DailyDocs</title>
  <style>
    body {
      margin: 0;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: #1f2933;
      background: #f7f8fa;
    }
    main {
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 2rem;
      box-sizing: border-box;
    }
    section {
      width: min(42rem, 100%);
    }
    h1 {
      margin: 0 0 0.75rem;
      font-size: clamp(2.5rem, 8vw, 5rem);
      line-height: 1;
    }
    p {
      margin: 0 0 1.5rem;
      max-width: 34rem;
      color: #52606d;
      font-size: 1.125rem;
      line-height: 1.6;
    }
    form {
      display: flex;
      gap: 0.75rem;
      align-items: center;
      max-width: 32rem;
      margin-bottom: 0.75rem;
    }
    .topic-lookup {
      position: relative;
      flex: 1;
      min-width: 0;
    }
    input {
      width: 100%;
      box-sizing: border-box;
      min-width: 0;
      padding: 0.75rem 0.875rem;
      border: 1px solid #cbd2d9;
      border-radius: 6px;
      font: inherit;
      background: #ffffff;
    }
    button {
      padding: 0.75rem 1rem;
      border: 0;
      border-radius: 6px;
      font: inherit;
      color: #ffffff;
      background: #1f2933;
      cursor: pointer;
    }
    ul {
      margin: 1.25rem 0 0;
      padding: 0;
      list-style: none;
      display: flex;
      flex-wrap: wrap;
      gap: 0.5rem;
    }
    a {
      color: #1f2933;
    }
    .lookup-status {
      margin: 0 0 1rem;
      color: #52606d;
      font-size: 0.95rem;
    }
    .topic-results {
      position: absolute;
      z-index: 2;
      top: calc(100% + 0.25rem);
      right: 0;
      left: 0;
      display: grid;
      gap: 0;
      margin: 0;
      padding: 0.25rem;
      list-style: none;
      border: 1px solid #cbd2d9;
      border-radius: 6px;
      background: #ffffff;
      box-shadow: 0 8px 24px rgba(31, 41, 51, 0.12);
    }
    .topic-results[hidden] {
      display: none;
    }
    .topic-option {
      padding: 0.65rem 0.75rem;
      border-radius: 4px;
      cursor: pointer;
    }
    .topic-option[aria-selected="true"] {
      color: #ffffff;
      background: #1f2933;
    }
  </style>
</head>
<body>
  <main>
    <section>
      <h1>DailyDocs</h1>
      <p>One documentation link per topic per day.</p>
      <form method="get" action="/read" id="topic-form">
        <div class="topic-lookup">
          <input
            name="topic"
            id="topic-input"
            autocomplete="off"
            placeholder="sqlite"
            aria-label="Topic"
            role="combobox"
            aria-autocomplete="list"
            aria-expanded="false"
            aria-controls="topic-results"
          >
          <ul class="topic-results" id="topic-results" role="listbox" hidden></ul>
        </div>
        <button type="submit" id="topic-button">View Reading</button>
      </form>
      <p class="lookup-status" id="topic-status"></p>
      {{if .Topics}}
      <ul>
        {{range .Topics}}<li><a href="/{{.Slug}}">{{.Name}}</a></li>{{end}}
      </ul>
      {{end}}
      <p><a href="/topics">All topics</a></p>
    </section>
  </main>
  <script>
    const input = document.getElementById("topic-input");
    const button = document.getElementById("topic-button");
    const status = document.getElementById("topic-status");
    const results = document.getElementById("topic-results");
    let controller = null;
    let matches = [];
    let activeIndex = -1;

    function exactMatch() {
      const value = input.value.trim().toLowerCase();
      return matches.find((topic) => topic.slug.toLowerCase() === value || topic.name.toLowerCase() === value);
    }

    function closeResults() {
      results.hidden = true;
      input.setAttribute("aria-expanded", "false");
      input.removeAttribute("aria-activedescendant");
      activeIndex = -1;
      updateActiveOption();
    }

    function openResults() {
      if (matches.length === 0) return;
      results.hidden = false;
      input.setAttribute("aria-expanded", "true");
    }

    function updateActiveOption() {
      Array.from(results.children).forEach((option, index) => {
        const selected = index === activeIndex;
        option.setAttribute("aria-selected", selected ? "true" : "false");
        if (selected) {
          input.setAttribute("aria-activedescendant", option.id);
        }
      });
      if (activeIndex < 0) {
        input.removeAttribute("aria-activedescendant");
      }
    }

    function selectMatch(index) {
      const topic = matches[index];
      if (!topic) return;
      input.value = topic.name;
      matches = [topic];
      closeResults();
      setLookupState();
    }

    function renderResults() {
      results.innerHTML = "";
      matches.forEach((topic, index) => {
        const option = document.createElement("li");
        option.id = "topic-option-" + index;
        option.className = "topic-option";
        option.setAttribute("role", "option");
        option.setAttribute("aria-selected", "false");
        option.textContent = topic.name;
        option.addEventListener("mousedown", (event) => {
          event.preventDefault();
          selectMatch(index);
        });
        results.appendChild(option);
      });
      activeIndex = -1;
      updateActiveOption();
      if (matches.length > 0) openResults();
      else closeResults();
    }

    function setLookupState() {
      const value = input.value.trim().toLowerCase();
      if (!value) {
        button.textContent = "View Reading";
        status.textContent = "";
        return;
      }

      if (exactMatch()) {
        button.textContent = "View Reading";
        status.textContent = "";
        return;
      }

      button.textContent = "Submit Documentation";
      status.textContent = matches.length > 0 ? "Select a topic from the list." : "No matching topic found.";
    }

    input.addEventListener("input", async () => {
      const value = input.value.trim();
      if (controller) controller.abort();
      if (!value) {
        matches = [];
        renderResults();
        setLookupState();
        return;
      }

      controller = new AbortController();
      try {
        const response = await fetch("/topics/search?q=" + encodeURIComponent(value), { signal: controller.signal });
        if (!response.ok) return;
        matches = await response.json();
        renderResults();
        setLookupState();
      } catch (error) {
        if (error.name !== "AbortError") status.textContent = "";
      }
    });

    input.addEventListener("keydown", (event) => {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        if (results.hidden) openResults();
        activeIndex = Math.min(activeIndex + 1, matches.length - 1);
        updateActiveOption();
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        activeIndex = Math.max(activeIndex - 1, 0);
        updateActiveOption();
      } else if (event.key === "Enter" && activeIndex >= 0 && !results.hidden) {
        event.preventDefault();
        selectMatch(activeIndex);
      } else if (event.key === "Escape") {
        closeResults();
      }
    });

    input.addEventListener("blur", () => {
      window.setTimeout(closeResults, 100);
    });
  </script>
</body>
</html>
`))

var topicsTemplate = template.Must(template.New("topics").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Topics - DailyDocs</title>
  <style>
    body {
      margin: 0;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: #1f2933;
      background: #f7f8fa;
    }
    main {
      width: min(42rem, 100%);
      margin: 0 auto;
      padding: 2rem;
      box-sizing: border-box;
    }
    h1 {
      margin: 0 0 1rem;
      font-size: clamp(2rem, 7vw, 4rem);
      line-height: 1;
    }
    ul {
      margin: 0 0 1.5rem;
      padding: 0;
      list-style: none;
      display: grid;
      gap: 0.65rem;
    }
    a {
      color: #1f2933;
    }
    p {
      margin: 0;
      color: #52606d;
      line-height: 1.6;
    }
  </style>
</head>
<body>
  <main>
    <h1>Topics</h1>
    {{if .Topics}}
    <ul>
      {{range .Topics}}<li><a href="/{{.Slug}}">{{.Name}}</a></li>{{end}}
    </ul>
    {{else}}
    <p>No topics yet.</p>
    {{end}}
    <p><a href="/">Home</a></p>
  </main>
</body>
</html>
`))

var submissionsTemplate = template.Must(template.New("submissions").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex">
  <title>Submissions - DailyDocs</title>
  <style>
    body {
      margin: 0;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: #1f2933;
      background: #f7f8fa;
    }
    main {
      width: min(48rem, 100%);
      margin: 0 auto;
      padding: 2rem;
      box-sizing: border-box;
    }
    h1 {
      margin: 0 0 0.75rem;
      font-size: clamp(2rem, 7vw, 4rem);
      line-height: 1;
    }
    p {
      margin: 0 0 1.5rem;
      color: #52606d;
      font-size: 1rem;
      line-height: 1.6;
    }
    form {
      display: grid;
      gap: 0.75rem;
      margin: 0 0 2rem;
      max-width: 36rem;
    }
    label {
      display: grid;
      gap: 0.35rem;
      color: #52606d;
      font-size: 0.95rem;
    }
    input {
      min-width: 0;
      padding: 0.75rem 0.875rem;
      border: 1px solid #cbd2d9;
      border-radius: 6px;
      font: inherit;
      background: #ffffff;
      color: #1f2933;
    }
    .honeypot {
      position: absolute;
      left: -10000px;
      width: 1px;
      height: 1px;
      overflow: hidden;
    }
    button {
      justify-self: start;
      padding: 0.75rem 1rem;
      border: 0;
      border-radius: 6px;
      font: inherit;
      color: #ffffff;
      background: #1f2933;
      cursor: pointer;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      background: #ffffff;
    }
    th, td {
      padding: 0.75rem;
      border-bottom: 1px solid #e4e7eb;
      text-align: left;
      vertical-align: top;
    }
    th {
      color: #52606d;
      font-size: 0.875rem;
      font-weight: 600;
    }
    a {
      color: #1f2933;
    }
  </style>
</head>
<body>
  <main>
    <h1>Documentation submissions</h1>
    <p>Submit a documentation source URL for a new or existing topic.</p>
    <form method="post" action="/submissions">
      <label>
        Documentation URL
        <input name="url" type="url" autocomplete="off" placeholder="https://sqlite.org/docs.html" required>
      </label>
      <label>
        Topic
        <input name="topic" autocomplete="off" placeholder="SQLite" value="{{.PrefillTopic}}">
      </label>
      <label class="honeypot">
        Website
        <input name="website" autocomplete="off" tabindex="-1">
      </label>
      <button type="submit">Submit</button>
    </form>

    {{if .Submissions}}
    <table>
      <thead>
        <tr>
          <th>Source</th>
          <th>Topic</th>
          <th>Status</th>
          <th>Requests</th>
          <th>Last submitted</th>
        </tr>
      </thead>
      <tbody>
        {{range .Submissions}}
        <tr>
          <td>{{.SourceHost}}</td>
          <td>{{if .SuggestedTopic}}{{.SuggestedTopic}}{{else}}-{{end}}</td>
          <td>{{.Status}}</td>
          <td>{{.RequestCount}}</td>
          <td>{{.LastSubmitted}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{else}}
    <p>No submissions yet.</p>
    {{end}}
    <p><a href="/">All topics</a></p>
  </main>
</body>
</html>
`))

var readingTemplate = template.Must(template.New("reading").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.TopicName}} - DailyDocs</title>
  <style>
    body {
      margin: 0;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: #1f2933;
      background: #f7f8fa;
    }
    main {
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 2rem;
      box-sizing: border-box;
    }
    article {
      width: min(42rem, 100%);
    }
    .date {
      margin: 0 0 0.5rem;
      color: #52606d;
      font-size: 0.95rem;
    }
    h1 {
      margin: 0 0 0.5rem;
      font-size: clamp(2.25rem, 7vw, 4.5rem);
      line-height: 1;
    }
    h2 {
      margin: 0 0 1rem;
      font-size: clamp(1.5rem, 4vw, 2.25rem);
      line-height: 1.15;
    }
    p {
      margin: 0 0 1.5rem;
      color: #52606d;
      font-size: 1.05rem;
      line-height: 1.6;
    }
    .badge {
      display: inline-block;
      margin-left: 0.5rem;
      font-size: 0.85rem;
      color: #1f2933;
    }
    a.button {
      display: inline-block;
      padding: 0.75rem 1rem;
      border-radius: 6px;
      color: #ffffff;
      background: #1f2933;
      text-decoration: none;
    }
    nav {
      margin-top: 1.5rem;
    }
    nav a {
      color: #52606d;
    }
  </style>
</head>
<body>
  <main>
    <article>
      <p class="date">{{.Date}}</p>
      <h1>{{.TopicName}}</h1>
      <h2>{{.Title}}</h2>
      <p>
        {{if .Source}}{{.Source}}{{else}}Documentation{{end}}
        {{if .Official}}<span class="badge">Official</span>{{end}}
        {{if .EstimatedMinutes}}<br>{{.EstimatedMinutes}} min{{end}}
      </p>
      <a class="button" href="{{.URL}}">Read</a>
      <nav><a href="/topics">All topics</a></nav>
      <nav><a href="/submissions?topic={{.TopicName}}">Suggest documentation source</a></nav>
    </article>
  </main>
</body>
</html>
`))

var adminLoginTemplate = template.Must(template.New("admin-login").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex">
  <title>Admin - DailyDocs</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1f2933; background: #f7f8fa; }
    main { width: min(28rem, 100%); margin: 0 auto; padding: 2rem; box-sizing: border-box; }
    h1 { margin: 0 0 1rem; font-size: 2rem; }
    form { display: grid; gap: 0.75rem; }
    input { padding: 0.75rem 0.875rem; border: 1px solid #cbd2d9; border-radius: 6px; font: inherit; }
    button { justify-self: start; padding: 0.75rem 1rem; border: 0; border-radius: 6px; font: inherit; color: #fff; background: #1f2933; cursor: pointer; }
    .error { color: #b42318; }
  </style>
</head>
<body>
  <main>
    <h1>Admin</h1>
    {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
    <form method="post" action="/admin/login">
      <label>
        Admin token
        <input name="token" type="password" autocomplete="current-password" autofocus>
      </label>
      <button type="submit">Sign in</button>
    </form>
  </main>
</body>
</html>
`))

var adminSubmissionsTemplate = template.Must(template.New("admin-submissions").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex">
  <title>Admin Submissions - DailyDocs</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1f2933; background: #f7f8fa; }
    main { width: min(68rem, 100%); margin: 0 auto; padding: 2rem; box-sizing: border-box; }
    h1 { margin: 0 0 1rem; font-size: 2rem; }
    table { width: 100%; border-collapse: collapse; background: #fff; }
    th, td { padding: 0.75rem; border-bottom: 1px solid #e4e7eb; text-align: left; vertical-align: top; }
    th { color: #52606d; font-size: 0.875rem; }
    a { color: #1f2933; }
    tr[data-href] { cursor: pointer; }
    tr[data-href]:hover { background: #f1f5f9; }
    tr[data-href]:focus { outline: 2px solid #1f2933; outline-offset: -2px; }
    .notice { color: #067647; }
    .error { color: #b42318; }
  </style>
</head>
<body>
  <main>
    <h1>Submissions</h1>
    {{if .Notice}}<p class="notice">{{.Notice}}</p>{{end}}
    {{if .Error}}<p class="error">{{.Error}}</p>{{end}}
    {{if .Submissions}}
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>Topic</th>
          <th>Source</th>
          <th>Status</th>
          <th>Requests</th>
          <th>Last submitted</th>
          <th>Error</th>
        </tr>
      </thead>
      <tbody>
        {{range .Submissions}}
        <tr data-href="/admin/submissions/{{.ID}}" tabindex="0">
          <td><a href="/admin/submissions/{{.ID}}">{{.ID}}</a></td>
          <td>{{if .SuggestedTopic}}{{.SuggestedTopic}}{{else}}-{{end}}</td>
          <td>{{.SourceHost}}</td>
          <td>{{.Status}}</td>
          <td>{{.RequestCount}}</td>
          <td>{{.LastSubmitted}}</td>
          <td>{{if .LastError}}{{.LastError}}{{else}}-{{end}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{else}}
    <p>No submissions.</p>
    {{end}}
  </main>
  <script>
    document.querySelectorAll("tr[data-href]").forEach((row) => {
      row.addEventListener("click", (event) => {
        if (event.target.closest("a, button")) return;
        window.location.href = row.dataset.href;
      });
      row.addEventListener("keydown", (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          window.location.href = row.dataset.href;
        }
      });
    });
  </script>
</body>
</html>
`))

var adminSubmissionDetailTemplate = template.Must(template.New("admin-submission-detail").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex">
  <title>Submission {{.Submission.ID}} - DailyDocs</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1f2933; background: #f7f8fa; }
    main { width: min(76rem, 100%); margin: 0 auto; padding: 2rem; box-sizing: border-box; }
    h1, h2 { margin: 0 0 1rem; }
    section { margin-top: 2rem; }
    dl { display: grid; grid-template-columns: 12rem 1fr; gap: 0.5rem 1rem; }
    dt { color: #52606d; }
    dd { margin: 0; }
    table { width: 100%; border-collapse: collapse; background: #fff; }
    th, td { padding: 0.75rem; border-bottom: 1px solid #e4e7eb; text-align: left; vertical-align: top; }
    th { color: #52606d; font-size: 0.875rem; }
    form { display: inline; margin-right: 0.5rem; }
    .source-form { display: grid; gap: 0.75rem; max-width: 32rem; margin: 0 0 1rem; }
    .source-form label { display: grid; gap: 0.35rem; color: #52606d; }
    input { min-width: 0; padding: 0.6rem 0.75rem; border: 1px solid #cbd2d9; border-radius: 6px; font: inherit; color: #1f2933; background: #fff; }
    button { padding: 0.6rem 0.85rem; border: 0; border-radius: 6px; font: inherit; color: #fff; background: #1f2933; cursor: pointer; }
    a { color: #1f2933; }
    .notice { color: #067647; }
    .error { color: #b42318; }
    .url { overflow-wrap: anywhere; }
  </style>
</head>
<body>
  <main>
    <p><a href="/admin/submissions">Submissions</a></p>
    <h1>Submission {{.Submission.ID}}</h1>
    {{if .Notice}}<p class="notice">{{.Notice}}</p>{{end}}
    {{if .Error}}<p class="error">{{.Error}}</p>{{end}}

    <form method="post" action="/admin/submissions/{{.Submission.ID}}/process">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button type="submit">Process</button>
    </form>
    <form method="post" action="/admin/submissions/{{.Submission.ID}}/activate">
      <input type="hidden" name="csrf" value="{{.CSRF}}">
      <button type="submit">Activate Candidates</button>
    </form>

    <section>
      <h2>Create Source</h2>
      <form class="source-form" method="post" action="/admin/submissions/{{.Submission.ID}}/create-source">
        <input type="hidden" name="csrf" value="{{.CSRF}}">
        <label>
          Topic slug
          <input name="topic_slug" value="{{.Submission.SuggestedSlug}}" required>
        </label>
        <label>
          Topic name
          <input name="topic_name" value="{{.Submission.SuggestedTopic}}">
        </label>
        <button type="submit">Create Source</button>
      </form>
    </section>

    <section>
      <h2>Details</h2>
      <dl>
        <dt>Topic</dt><dd>{{if .Submission.SuggestedTopic}}{{.Submission.SuggestedTopic}}{{else}}-{{end}}</dd>
        <dt>Source</dt><dd>{{.Submission.SourceHost}}</dd>
        <dt>Status</dt><dd>{{.Submission.Status}}</dd>
        <dt>Requests</dt><dd>{{.Submission.RequestCount}}</dd>
        <dt>Submitted URL</dt><dd class="url">{{.Submission.SubmittedURL}}</dd>
        <dt>Normalized URL</dt><dd class="url">{{.Submission.NormalizedURL}}</dd>
        <dt>Last submitted</dt><dd>{{.Submission.LastSubmitted}}</dd>
        <dt>Last error</dt><dd>{{if .Submission.LastError}}{{.Submission.LastError}}{{else}}-{{end}}</dd>
      </dl>
    </section>

    <section>
      <h2>Sources</h2>
      {{if .Submission.Sources}}
      <table>
        <thead><tr><th>ID</th><th>Topic</th><th>Status</th><th>Type</th><th>URL</th><th>Last processed</th><th>Error</th><th>Action</th></tr></thead>
        <tbody>
          {{range .Submission.Sources}}
          <tr>
            <td>{{.ID}}</td>
            <td>{{.TopicSlug}}</td>
            <td>{{.Status}}</td>
            <td>{{.SourceType}}</td>
            <td class="url">{{.NormalizedURL}}</td>
            <td>{{.LastProcessedAt}}</td>
            <td>{{.LastError}}</td>
            <td>
              <form method="post" action="/admin/submissions/{{$.Submission.ID}}/process-source">
                <input type="hidden" name="csrf" value="{{$.CSRF}}">
                <input type="hidden" name="source_id" value="{{.ID}}">
                <button type="submit">Process Source</button>
              </form>
            </td>
          </tr>
          {{end}}
        </tbody>
      </table>
      {{else}}<p>No sources.</p>{{end}}
    </section>

    <section>
      <h2>Runs</h2>
      {{if .Submission.Runs}}
      <table>
        <thead><tr><th>ID</th><th>Status</th><th>Started</th><th>Completed</th><th>Discovered</th><th>Crawled</th><th>Eligible</th><th>Rejected</th><th>Failed</th><th>Error</th></tr></thead>
        <tbody>
          {{range .Submission.Runs}}
          <tr><td>{{.ID}}</td><td>{{.Status}}</td><td>{{.StartedAt}}</td><td>{{.CompletedAt}}</td><td>{{.DiscoveredCount}}</td><td>{{.CrawledCount}}</td><td>{{.EligibleCount}}</td><td>{{.RejectedCount}}</td><td>{{.FailureCount}}</td><td>{{if .Error}}{{.Error}}{{else}}-{{end}}</td></tr>
          {{end}}
        </tbody>
      </table>
      {{else}}<p>No runs.</p>{{end}}
    </section>

    <section>
      <h2>Candidates</h2>
      {{if .Submission.Candidates}}
      <table>
        <thead><tr><th>ID</th><th>Score</th><th>Gate</th><th>Status</th><th>Stage</th><th>Min</th><th>Class</th><th>Title</th><th>URL</th><th>Reason</th></tr></thead>
        <tbody>
          {{range .Submission.Candidates}}
          <tr><td>{{.ID}}</td><td>{{.Score}}</td><td>{{.Gate}}</td><td>{{.Status}}</td><td>{{.RejectStage}}</td><td>{{.EstimatedMinutes}}</td><td>{{.Classification}}</td><td>{{.Title}}</td><td class="url">{{.URL}}</td><td>{{.Reason}}</td></tr>
          {{end}}
        </tbody>
      </table>
      {{else}}<p>No candidates.</p>{{end}}
    </section>
  </main>
</body>
</html>
`))
