/* Safe DOM construction helpers — avoids unsafe HTML insertion. */

window.el = function (tag, attrs, children) {
  const node = document.createElement(tag);
  if (attrs) {
    for (const k in attrs) {
      if (k === "class") node.className = attrs[k];
      else if (k === "text") node.textContent = attrs[k];
      else if (k.startsWith("on") && typeof attrs[k] === "function") node.addEventListener(k.slice(2), attrs[k]);
      else node.setAttribute(k, attrs[k]);
    }
  }
  if (children) {
    for (const c of [].concat(children)) {
      if (c == null || c === false) continue;
      if (typeof c === "string" || typeof c === "number") node.appendChild(document.createTextNode(String(c)));
      else node.appendChild(c);
    }
  }
  return node;
};

window.clear = function (n) {
  while (n.firstChild) n.removeChild(n.firstChild);
  return n;
};
