#!/usr/bin/env node
/**
 * Generate Cutable's editable Excalidraw sources plus matching SVG/PNG exports.
 *
 * The scene data is the source of truth. Both formats are rendered from the
 * same nodes and edges so documentation images cannot drift from the editable
 * diagrams.
 */

import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import sharp from "../apps/frontend/node_modules/sharp/dist/index.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sourceDir = path.join(root, "docs/architecture/diagrams");
const publicDir = path.join(root, "apps/frontend/public/docs");
const width = 1600;
const height = 900;

const colors = {
  canvas: "#f7f5ef",
  ink: "#17211e",
  muted: "#66706c",
  line: "#43524d",
  panel: "#fffefb",
  green: "#dce9e3",
  greenStrong: "#2f5e50",
  orange: "#ff8a1f",
  pink: "#e63883",
  violet: "#635bff",
  blue: "#dfe8ff",
  amber: "#fff0d3",
  red: "#fee2e2",
};

const escapeXml = (value) =>
  value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");

function scene(title, kicker) {
  return { title, kicker, nodes: [], edges: [], notes: [] };
}

function node(s, id, x, y, w, h, label, detail, tone = "panel") {
  s.nodes.push({ id, x, y, w, h, label, detail, tone });
  return id;
}

function edge(s, from, to, label = "", style = "solid") {
  s.edges.push({ from, to, label, style });
}

function note(s, x, y, text, tone = "muted") {
  s.notes.push({ x, y, text, tone });
}

const diagrams = [];

{
  const s = scene("System context", "01 / request path + operating boundary");
  node(s, "person", 70, 255, 180, 118, "BUILDER", "browser", "amber");
  node(s, "web", 345, 205, 250, 220, "NEXT.JS 16", "Vercel\nUI · auth entry\neditor · preview", "panel");
  node(s, "api", 710, 205, 250, 220, "GO API", "Myprod / Nomad\nHTTP + WebSocket\npolicy + orchestration", "green");
  node(s, "neon", 1090, 95, 300, 135, "NEON POSTGRES", "users · projects · files\nmessages · attachments · quota", "blue");
  node(s, "openrouter", 1090, 300, 300, 135, "OPENROUTER", "planning + tool-call inference\nmultimodal prompt input", "amber");
  node(s, "e2b", 1090, 505, 300, 135, "E2B SANDBOX", "isolated React workspace\nfilesystem · npm · Vite :5173", "green");
  node(s, "github", 345, 650, 250, 110, "GITHUB", "source + Actions", "panel");
  node(s, "ghcr", 710, 650, 250, 110, "GHCR", "immutable multi-arch image", "panel");

  edge(s, "person", "web", "HTTPS");
  edge(s, "web", "api", "cookie + WSS");
  edge(s, "api", "neon", "SQL / TLS");
  edge(s, "api", "openrouter", "chat completions");
  edge(s, "api", "e2b", "sandbox API");
  edge(s, "github", "web", "Vercel build", "dashed");
  edge(s, "github", "ghcr", "Actions build", "dashed");
  edge(s, "ghcr", "api", "Nomad deploy", "dashed");
  note(s, 1075, 710, "Generated apps execute in E2B — never on the Go host.", "greenStrong");
  diagrams.push(["system-context", s]);
}

{
  const s = scene("One user build", "02 / authenticated product flow");
  node(s, "auth", 60, 120, 230, 120, "1  SIGN IN", "password or Google OAuth", "panel");
  node(s, "prompt", 360, 120, 230, 120, "2  DESCRIBE", "prompt + validated text/image refs", "amber");
  node(s, "project", 660, 120, 230, 120, "3  PROJECT", "persist owner + initial state", "blue");
  node(s, "socket", 960, 120, 230, 120, "4  CONNECT", "authenticated project WebSocket", "green");
  node(s, "gate", 1260, 105, 250, 150, "5  RUN GATE", "own keys?\nOR atomically claim demo 1/2", "amber");
  node(s, "agent", 1085, 365, 250, 140, "6  AGENT", "plan → tools → build → serve", "green");
  node(s, "events", 745, 375, 240, 120, "LIVE EVENTS", "stage · thinking · tool status", "panel");
  node(s, "preview", 405, 375, 240, 120, "PREVIEW", "E2B URL + Vite HMR", "blue");
  node(s, "edit", 65, 375, 240, 120, "ITERATE", "chat or visual/code inspection", "panel");
  node(s, "byok", 1085, 650, 250, 105, "BYOK PATH", "sessionStorage → one WSS run\nnot written to PostgreSQL", "red");
  node(s, "demo", 1370, 650, 170, 105, "DEMO PATH", "server-held keys\n2 claimed runs", "amber");

  edge(s, "auth", "prompt");
  edge(s, "prompt", "project");
  edge(s, "project", "socket");
  edge(s, "socket", "gate");
  edge(s, "gate", "agent", "allowed");
  edge(s, "agent", "events", "stream");
  edge(s, "events", "preview");
  edge(s, "preview", "edit");
  edge(s, "edit", "socket", "next prompt", "dashed");
  edge(s, "byok", "gate", "both keys", "dashed");
  edge(s, "demo", "gate", "quota", "dashed");
  note(s, 64, 745, "Ownership is checked before project data or WebSocket access is granted.");
  diagrams.push(["user-build-flow", s]);
}

{
  const s = scene("AI execution loop", "03 / model proposes, Go controls, sandbox executes");
  node(s, "input", 65, 120, 250, 125, "RUN INPUT", "project prompt + attachments\npersisted project files", "amber");
  node(s, "restore", 395, 120, 250, 125, "SANDBOX SESSION", "reconnect or create\nrestore files from Neon", "green");
  node(s, "plan", 725, 120, 250, 125, "PLAN", "OpenRouter completion\nordered implementation steps", "blue");
  node(s, "think", 1055, 120, 250, 125, "TOOL DECISION", "OpenRouter returns one tool call\nparallel calls disabled", "amber");
  node(s, "policy", 1055, 390, 250, 145, "GO EXECUTOR", "parse args · constrain app path\nselect allow-listed operation", "green");
  node(s, "sandbox", 725, 390, 250, 145, "E2B WORKSPACE", "file ops · shell · npm\nbuild · Vite preview", "panel");
  node(s, "persist", 395, 390, 250, 145, "PERSIST + EMIT", "upsert project files in Neon\nstream tool result over WSS", "blue");
  node(s, "done", 65, 390, 250, 145, "EXIT CONDITION", "model returns no tool calls\nor maximum step/error", "panel");
  node(s, "tools", 395, 665, 910, 105, "ALLOW-LISTED TOOL SURFACE", "write / read / delete / move files   ·   list tree   ·   execute command   ·   add/check dependencies   ·   test build   ·   start server", "panel");

  edge(s, "input", "restore");
  edge(s, "restore", "plan");
  edge(s, "plan", "think");
  edge(s, "think", "policy", "tool name + JSON args");
  edge(s, "policy", "sandbox", "execute");
  edge(s, "sandbox", "persist", "result + changed files");
  edge(s, "persist", "think", "tool result → next step", "dashed");
  edge(s, "think", "done", "no tool calls");
  edge(s, "tools", "policy", "only these operations", "dashed");
  note(s, 65, 815, "The model does not receive host shell access. Commands run with cwd=/home/user/react-app inside E2B.");
  diagrams.push(["ai-agent-loop", s]);
}

{
  const s = scene("Trust, data and delivery", "04 / boundaries + reproducible production path");
  node(s, "browser", 60, 105, 300, 190, "BROWSER TRUST ZONE", "Secure HttpOnly auth cookie\nBYOK values: sessionStorage only\nattachments checked before upload", "amber");
  node(s, "edge", 470, 105, 300, 190, "PUBLIC EDGE", "Vercel frontend\nTLS / HTTPS\nexact backend origin", "panel");
  node(s, "control", 880, 105, 300, 190, "CONTROL PLANE", "Go API on Myprod\nJWT + owner checks\nrequest limits + tool allow-list", "green");
  node(s, "data", 1290, 105, 250, 190, "DATA PLANE", "Neon TLS\nrelational ownership\ncascade deletion", "blue");
  node(s, "source", 60, 510, 250, 125, "SOURCE", "GitHub main", "panel");
  node(s, "ci", 390, 510, 250, 125, "CI", "test + Docker buildx\namd64 / arm64", "panel");
  node(s, "registry", 720, 510, 250, 125, "REGISTRY", "GHCR digest", "blue");
  node(s, "scheduler", 1050, 510, 250, 125, "SCHEDULER", "Myprod renders Nomad\njob + Traefik route", "green");
  node(s, "runtime", 1380, 510, 160, 125, "RUNTIME", "distroless\nnonroot", "green");
  node(s, "secret", 1050, 720, 490, 90, "HOST SECRET FILE", "0400 · uid 65532 · read-only mount at /run/secrets/cutable.env", "red");

  edge(s, "browser", "edge", "HTTPS");
  edge(s, "edge", "control", "HTTPS / WSS");
  edge(s, "control", "data", "SQL / TLS");
  edge(s, "source", "ci", "push");
  edge(s, "ci", "registry", "publish immutable digest");
  edge(s, "registry", "scheduler", "deploy spec");
  edge(s, "scheduler", "runtime", "Nomad");
  edge(s, "secret", "runtime", "read-only mount", "dashed");
  note(s, 62, 365, "External execution boundary: OpenRouter receives inference input; E2B receives generated workspace operations.");
  diagrams.push(["trust-and-delivery", s]);
}

function center(node) {
  return { x: node.x + node.w / 2, y: node.y + node.h / 2 };
}

function connection(from, to) {
  const a = center(from);
  const b = center(to);
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  if (Math.abs(dx) >= Math.abs(dy)) {
    return {
      x1: dx >= 0 ? from.x + from.w : from.x,
      y1: a.y,
      x2: dx >= 0 ? to.x : to.x + to.w,
      y2: b.y,
    };
  }
  return {
    x1: a.x,
    y1: dy >= 0 ? from.y + from.h : from.y,
    x2: b.x,
    y2: dy >= 0 ? to.y : to.y + to.h,
  };
}

function wrapLines(value, max = 34) {
  const output = [];
  for (const explicit of value.split("\n")) {
    const words = explicit.split(" ");
    let line = "";
    for (const word of words) {
      if (line && `${line} ${word}`.length > max) {
        output.push(line);
        line = word;
      } else {
        line = line ? `${line} ${word}` : word;
      }
    }
    output.push(line);
  }
  return output;
}

function renderSvg(s) {
  const nodeById = new Map(s.nodes.map((item) => [item.id, item]));
  const edgeMarkup = s.edges
    .map((item) => {
      const points = connection(nodeById.get(item.from), nodeById.get(item.to));
      const midX = (points.x1 + points.x2) / 2;
      const midY = (points.y1 + points.y2) / 2;
      const dash = item.style === "dashed" ? 'stroke-dasharray="10 10"' : "";
      return `<g>
        <path d="M ${points.x1} ${points.y1} L ${points.x2} ${points.y2}" fill="none" stroke="${colors.line}" stroke-width="3" ${dash} marker-end="url(#arrow)" />
        ${item.label ? `<rect x="${midX - item.label.length * 3.8 - 8}" y="${midY - 14}" width="${item.label.length * 7.6 + 16}" height="24" rx="8" fill="${colors.canvas}"/><text x="${midX}" y="${midY + 4}" text-anchor="middle" class="edge">${escapeXml(item.label)}</text>` : ""}
      </g>`;
    })
    .join("\n");
  const nodeMarkup = s.nodes
    .map((item) => {
      const fill = colors[item.tone] ?? colors.panel;
      const lines = wrapLines(item.detail, Math.max(18, Math.floor((item.w - 44) / 8.2)));
      return `<g>
        <rect x="${item.x}" y="${item.y}" width="${item.w}" height="${item.h}" rx="18" fill="${fill}" stroke="${colors.ink}" stroke-width="2.5"/>
        <circle cx="${item.x + 24}" cy="${item.y + 27}" r="6" fill="${item.tone === "panel" ? colors.greenStrong : colors.pink}"/>
        <text x="${item.x + 42}" y="${item.y + 33}" class="label">${escapeXml(item.label)}</text>
        ${lines.map((line, index) => `<text x="${item.x + 22}" y="${item.y + 67 + index * 24}" class="detail">${escapeXml(line)}</text>`).join("")}
      </g>`;
    })
    .join("\n");
  const noteMarkup = s.notes
    .map((item) => `<text x="${item.x}" y="${item.y}" fill="${colors[item.tone] ?? colors.muted}" class="note">${escapeXml(item.text)}</text>`)
    .join("\n");

  return `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}" role="img" aria-labelledby="title desc">
    <title id="title">${escapeXml(s.title)}</title>
    <desc id="desc">${escapeXml(s.kicker)}</desc>
    <defs>
      <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="${colors.line}"/></marker>
      <filter id="shadow" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="4" stdDeviation="7" flood-opacity=".08"/></filter>
    </defs>
    <style>
      text { font-family: Inter, ui-sans-serif, system-ui, -apple-system, sans-serif; fill: ${colors.ink}; }
      .title { font-size: 39px; font-weight: 700; letter-spacing: -1.2px; }
      .kicker { font-size: 16px; font-weight: 650; letter-spacing: 1.6px; fill: ${colors.greenStrong}; }
      .label { font-size: 17px; font-weight: 750; letter-spacing: .5px; }
      .detail { font-size: 16px; fill: ${colors.muted}; }
      .edge { font-size: 13px; font-weight: 650; fill: ${colors.line}; }
      .note { font-size: 16px; font-weight: 600; }
    </style>
    <rect width="1600" height="900" fill="${colors.canvas}"/>
    <path d="M0 0 H1600" stroke="${colors.orange}" stroke-width="10"/>
    <text x="64" y="62" class="kicker">${escapeXml(s.kicker.toUpperCase())}</text>
    <text x="64" y="104" class="title">${escapeXml(s.title)}</text>
    <g filter="url(#shadow)">${edgeMarkup}${nodeMarkup}</g>
    ${noteMarkup}
    <text x="1535" y="858" text-anchor="end" class="edge">CUTABLE / VERIFIED FROM REPOSITORY</text>
  </svg>`;
}

function excalidrawElements(s) {
  const nodeById = new Map(s.nodes.map((item) => [item.id, item]));
  const base = (id, type, x, y, w, h) => ({
    id,
    type,
    x,
    y,
    width: w,
    height: h,
    angle: 0,
    strokeColor: colors.ink,
    backgroundColor: "transparent",
    fillStyle: "solid",
    strokeWidth: 2,
    strokeStyle: "solid",
    roughness: 1,
    opacity: 100,
    groupIds: [],
    frameId: null,
    index: `a${id}`,
    roundness: type === "rectangle" ? { type: 3 } : null,
    seed: [...id].reduce((sum, char) => sum + char.charCodeAt(0), 1),
    version: 1,
    versionNonce: 1,
    isDeleted: false,
    boundElements: null,
    updated: 1,
    link: null,
    locked: false,
  });
  const elements = [];
  elements.push({
    ...base("title", "text", 64, 45, 500, 45),
    text: s.title,
    originalText: s.title,
    fontSize: 38,
    fontFamily: 1,
    textAlign: "left",
    verticalAlign: "top",
    baseline: 36,
    lineHeight: 1.25,
  });
  elements.push({
    ...base("kicker", "text", 64, 20, 600, 24),
    text: s.kicker.toUpperCase(),
    originalText: s.kicker.toUpperCase(),
    fontSize: 16,
    fontFamily: 3,
    strokeColor: colors.greenStrong,
    textAlign: "left",
    verticalAlign: "top",
    baseline: 16,
    lineHeight: 1.25,
  });
  for (const item of s.nodes) {
    elements.push({
      ...base(item.id, "rectangle", item.x, item.y, item.w, item.h),
      backgroundColor: colors[item.tone] ?? colors.panel,
    });
    const value = `${item.label}\n${item.detail}`;
    elements.push({
      ...base(`${item.id}-text`, "text", item.x + 18, item.y + 18, item.w - 36, item.h - 36),
      text: value,
      originalText: value,
      fontSize: 17,
      fontFamily: 3,
      textAlign: "left",
      verticalAlign: "top",
      baseline: 16,
      lineHeight: 1.35,
    });
  }
  s.edges.forEach((item, index) => {
    const p = connection(nodeById.get(item.from), nodeById.get(item.to));
    const dx = p.x2 - p.x1;
    const dy = p.y2 - p.y1;
    elements.push({
      ...base(`edge-${index}`, "arrow", p.x1, p.y1, Math.abs(dx), Math.abs(dy)),
      strokeColor: colors.line,
      strokeStyle: item.style === "dashed" ? "dashed" : "solid",
      points: [[0, 0], [dx, dy]],
      lastCommittedPoint: null,
      startBinding: null,
      endBinding: null,
      startArrowhead: null,
      endArrowhead: "arrow",
      elbowed: false,
    });
  });
  s.notes.forEach((item, index) => {
    elements.push({
      ...base(`note-${index}`, "text", item.x, item.y, 900, 24),
      text: item.text,
      originalText: item.text,
      fontSize: 16,
      fontFamily: 3,
      strokeColor: colors[item.tone] ?? colors.muted,
      textAlign: "left",
      verticalAlign: "top",
      baseline: 16,
      lineHeight: 1.25,
    });
  });
  return elements;
}

await fs.mkdir(sourceDir, { recursive: true });
await fs.mkdir(publicDir, { recursive: true });

for (const [slug, data] of diagrams) {
  const svg = renderSvg(data).replace(/[ \t]+\n/g, "\n");
  const excalidraw = {
    type: "excalidraw",
    version: 2,
    source: "https://excalidraw.com",
    elements: excalidrawElements(data),
    appState: {
      gridSize: 20,
      gridStep: 5,
      gridModeEnabled: false,
      viewBackgroundColor: colors.canvas,
    },
    files: {},
  };

  await fs.writeFile(path.join(sourceDir, `${slug}.excalidraw`), `${JSON.stringify(excalidraw, null, 2)}\n`);
  await fs.writeFile(path.join(sourceDir, `${slug}.svg`), svg);
  await sharp(Buffer.from(svg)).png({ compressionLevel: 9 }).toFile(path.join(sourceDir, `${slug}.png`));
  await fs.copyFile(path.join(sourceDir, `${slug}.svg`), path.join(publicDir, `${slug}.svg`));
  await fs.copyFile(path.join(sourceDir, `${slug}.png`), path.join(publicDir, `${slug}.png`));
}

console.log(`Generated ${diagrams.length} editable Excalidraw scenes and SVG/PNG export pairs.`);
