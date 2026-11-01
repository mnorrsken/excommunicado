"use strict";

const TEXT = {
  en: {
    scoring: "Scoring…",
    noMail: "Select a mail to score it.",
    empty: "This mail has no text to score.",
    readFailed: "Outlook could not read this mail.",
    down: "The scoring service is not answering. Is the container running?",
    wordsTitle: "Buzzwords found",
    truncated: "Long mail: the model only read the start of it.",
    pasteLabel: "Paste a mail here",
    pasteButton: "Score",
    bands: ["Plain speech", "Mild jargon", "Heavy jargon", "Corporate bullshit", "Full buzzword bingo"],
  },
  sv: {
    scoring: "Betygsätter…",
    noMail: "Välj ett mejl för att betygsätta det.",
    empty: "Det här mejlet har ingen text att betygsätta.",
    readFailed: "Outlook kunde inte läsa mejlet.",
    down: "Tjänsten svarar inte. Är containern igång?",
    wordsTitle: "Hittade modeord",
    truncated: "Långt mejl: modellen läste bara början.",
    pasteLabel: "Klistra in ett mejl här",
    pasteButton: "Betygsätt",
    bands: ["Klarspråk", "Lite jargong", "Mycket jargong", "Företagsbullshit", "Fullt modeordsbingo"],
  },
};

let text = TEXT.en;
let running = null; // AbortController of the request in progress
const cache = new Map(); // scores per mail while the pane is open

const $ = (id) => document.getElementById(id);

function setLanguage(tag) {
  text = (tag || "").toLowerCase().startsWith("sv") ? TEXT.sv : TEXT.en;
  $("words-title").textContent = text.wordsTitle;
  $("paste-label").textContent = text.pasteLabel;
  $("paste-button").textContent = text.pasteButton;
}

function showStatus(message) {
  $("status").textContent = message;
  $("result").hidden = true;
}

function band(score) {
  if (score <= 2) return { index: 0, cls: "low" };
  if (score <= 4) return { index: 1, cls: "mid" };
  if (score <= 6) return { index: 2, cls: "mid" };
  if (score <= 8) return { index: 3, cls: "high" };
  return { index: 4, cls: "high" };
}

function showResult(res) {
  const b = band(res.score);
  $("status").textContent = "";
  $("result").hidden = false;
  $("result").className = b.cls;
  $("score").textContent = res.score;
  $("meter-fill").style.width = res.score * 10 + "%";
  $("verdict").textContent = text.bands[b.index];
  $("note").textContent = res.truncated ? text.truncated : "";
  // Buzzwords come from the mail, so they only ever go in as plain text.
  const list = $("words");
  list.replaceChildren();
  $("words-title").hidden = res.buzzwords.length === 0;
  for (const word of res.buzzwords) {
    const li = document.createElement("li");
    li.textContent = word;
    list.append(li);
  }
}

async function score(subject, body, cacheKey) {
  if (running) running.abort();
  const controller = new AbortController();
  running = controller;
  showStatus(text.scoring);
  try {
    const resp = await fetch("api/score", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ subject, body }),
      signal: controller.signal,
    });
    if (running !== controller) return; // the user moved on to another mail
    if (resp.status === 422) return showStatus(text.empty);
    if (!resp.ok) return showStatus(text.down);
    const res = await resp.json();
    if (running !== controller) return;
    if (cacheKey) cache.set(cacheKey, res);
    showResult(res);
  } catch (err) {
    if (err.name !== "AbortError" && running === controller) showStatus(text.down);
  }
}

function scoreSelectedMail() {
  if (running) running.abort();
  running = null;
  const item = Office.context.mailbox.item;
  if (!item) return showStatus(text.noMail);
  const known = cache.get(item.itemId);
  if (known) return showResult(known);
  const key = item.itemId;
  item.body.getAsync(Office.CoercionType.Text, (result) => {
    // A slow read can finish after the user has already selected another mail.
    if (Office.context.mailbox.item?.itemId !== key) return;
    if (result.status !== Office.AsyncResultStatus.Succeeded) return showStatus(text.readFailed);
    score(item.subject, result.value, key);
  });
}

function startPasteMode() {
  $("paste").hidden = false;
  $("paste").addEventListener("submit", (e) => {
    e.preventDefault();
    score("", $("paste-text").value);
  });
}

function start() {
  if (typeof Office === "undefined") {
    // office.js could not be loaded, and it is only needed inside Outlook.
    setLanguage(navigator.language);
    return startPasteMode();
  }
  Office.onReady((info) => {
    if (info.host !== Office.HostType.Outlook) {
      setLanguage(navigator.language);
      return startPasteMode();
    }
    setLanguage(Office.context.displayLanguage);
    // Needs Mailbox 1.5. Older Outlook closes the pane when you change mail instead.
    if (Office.context.requirements.isSetSupported("Mailbox", "1.5")) {
      Office.context.mailbox.addHandlerAsync(Office.EventType.ItemChanged, scoreSelectedMail);
    }
    scoreSelectedMail();
  });
}

start();
